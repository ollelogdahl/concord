package fz

import (
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"time"

	"github.com/stretchr/testify/assert"
)

type Action[S any, P any] struct {
	Name string
	Gen  func(*S) []P
	Exec func(*S, P)
}

func (a *Action[S, P]) generate(s *S) []concreteAction[S] {
	params := a.Gen(s)
	concr := make([]concreteAction[S], len(params))
	for i, p := range params {
		concr[i] = concreteAction[S]{
			name: fmt.Sprintf("%s %+v", a.Name, p),
			exec: func(s *S) {
				a.Exec(s, p)
			},
		}
	}
	return concr
}

type actionTypeWrapper[S any] interface {
	generate(*S) []concreteAction[S]
}

type concreteAction[S any] struct {
	name string
	exec func(*S)
}

type Invariant[S any] struct {
	Name  string
	Check func(assert.TestingT, *S)
}

type Fuzzer[S any] struct {
	logger              *log.Logger
	actions             []actionTypeWrapper[S]
	invariants          []Invariant[S]
	iteration           int
	consecutiveStutters int
	rng                 *rand.Rand
	ct                  CrashT

	startTime     time.Time
	maxDuration   time.Duration
	actionTimeout time.Duration
}

func NewFuzzer[S any](r *rand.Rand, maxDuration time.Duration) *Fuzzer[S] {
	return &Fuzzer[S]{
		logger:        log.Default(),
		actions:       make([]actionTypeWrapper[S], 0),
		invariants:    make([]Invariant[S], 0),
		iteration:     0,
		rng:           r,
		ct:            CrashT{},
		maxDuration:   maxDuration,
		actionTimeout: 10 * time.Second,
	}
}

func AddAction[S any, P any](f *Fuzzer[S], name string, gen func(*S) []P, exec func(*S, P)) {
	at := &Action[S, P]{
		Name: name,
		Gen:  gen,
		Exec: exec,
	}
	f.actions = append(f.actions, at)
}

func (f *Fuzzer[S]) AddInvariant(name string, check func(assert.TestingT, *S)) {
	i := Invariant[S]{name, check}
	f.invariants = append(f.invariants, i)
}

func (f *Fuzzer[S]) CheckInvariants(s *S) {
	for _, inv := range f.invariants {
		f.ct.where = fmt.Sprintf(" in invariant %s", inv.Name)
		inv.Check(&f.ct, s)
	}
}

func (f *Fuzzer[S]) Iteration(s *S) {
	f.iteration++
	enabled := make([]concreteAction[S], 0)
	for _, a := range f.actions {
		enabled = append(enabled, a.generate(s)...)
	}

	if len(enabled) == 0 {
		f.logger.Printf("iteration %v: stuttering\n", f.iteration)
		f.consecutiveStutters++

		if f.consecutiveStutters >= 5 {
			f.ct.Errorf("fuzzer stuttered consecutively for %v iterations; aborting", f.consecutiveStutters)
		}

		return
	}
	f.consecutiveStutters = 0

	// pick random
	selectedAction := enabled[rand.IntN(len(enabled))]

	runtime := time.Since(f.startTime)
	f.logger.Printf("(%s) iteration %v: executing %s\n", runtime, f.iteration, selectedAction.name)

	doneChan := make(chan bool, 1)

	go func() {
		selectedAction.exec(s)
		select {
		case doneChan <- true:
		default:
		}
	}()

	select {
	case _ = <-doneChan:
	case <-time.After(f.actionTimeout):
		f.ct.where = ""
		f.ct.Errorf("action %s timed out after %s", selectedAction.name, f.actionTimeout)
	}

	f.CheckInvariants(s)
}

func (f *Fuzzer[S]) Run(s *S) {
	f.startTime = time.Now()
	f.ct.startTime = f.startTime

	endTime := f.startTime.Add(f.maxDuration)

	for time.Now().Before(endTime) {
		f.Iteration(s)
	}
}

// CrashT implements testing.TB and assert.TestingT.
// It is designed to panic immediately on any assertion failure.
type CrashT struct {
	where     string
	startTime time.Time
}

// Crash immediately on non-fatal assertion failure (e.g., assert.Equal).
func (t *CrashT) Errorf(format string, args ...interface{}) {
	errorMsg := fmt.Sprintf(format, args...)

	fmt.Printf("\n[FUZZER CRASH] (%s) fatal assertion failure%s: %s\n", time.Since(t.startTime), t.where, errorMsg)
	os.Exit(1)
}
