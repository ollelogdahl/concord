package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
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
	fmt.Println("Server started at", *bindAddr)

	if *joinAddr == "" {
		server.Create()
		fmt.Println("Cluster created")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fmt.Println("Joining cluster...")

		err := server.Join(ctx, *joinAddr)
		if err != nil {
			panic(err)
		}
		fmt.Println("Joined cluster")
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter keys to lookup (Ctrl+D to exit):")
	for scanner.Scan() {
		key := scanner.Text()
		if key == "" {
			continue
		}

		// Perform lookup
		value, err := server.Lookup([]byte(key))
		if err != nil {
			fmt.Printf("Error looking up '%s': %v\n", key, err)
		} else {
			fmt.Printf("'%s' -> %s\n", key, value.Name)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
	}
}
