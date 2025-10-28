package main

import (
	"context"
	"flag"
	"time"

	"github.com/ollelogdahl/concord"
)

func main() {
	name := flag.String("name", "cord0", "name of the server")
	bindAddr := flag.String("addr", ":8000", "address to bind to")

	joinAddr := flag.String("join", "", "address of a server to join")

	flag.Parse()
	advAddr := "localhost" + *bindAddr

	// Initialize the server
	server := concord.New(concord.Config{
		Name:     *name,
		BindAddr: *bindAddr,
		AdvAddr:  advAddr,
	})

	err := server.Start()
	if err != nil {
		panic(err)
	}

	if *joinAddr == "" {
		server.Create()
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := server.Join(ctx, *joinAddr)
		if err != nil {
			panic(err)
		}
	}

	for {
		time.Sleep(10 * time.Second)
	}
}
