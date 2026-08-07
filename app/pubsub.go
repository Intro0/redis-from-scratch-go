package main

import (
	"fmt"
	"net"
)

// acknowledges subscription to a channel
func handleSubscribe(conn net.Conn, args []string) {
	channel := args[1]

	response := fmt.Sprintf(
		"*3\r\n$9\r\nsubscribe\r\n$%d\r\n%s\r\n:1\r\n",
		len(channel),
		channel,
	)

	conn.Write([]byte(response))
}
