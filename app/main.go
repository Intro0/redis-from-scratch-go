package main

import (
	"flag"
	"fmt"
	"net"
	"os"
)

func main() {

	// init storage used by all client connections
	storage := &Storage{
		data: make(map[string]Value),
	}
	pubsub := newPubSub()

	// grab port from --port flag, 6379 default
	port := flag.Int("port", 6379, "Port to listen on")
	flag.Parse()

	// listen for TCP connections on selected port and all network interfaces
	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}

	// handles multiple connections concurrently using goroutine
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go handleConnection(conn, storage, pubsub)
	}
}
