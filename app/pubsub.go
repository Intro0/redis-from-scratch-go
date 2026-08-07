package main

import (
	"fmt"
	"net"
)

// acknowledges subscription to a channel
func handleSubscribe(conn net.Conn, args []string, subscriptions map[string]struct{}) {
	channel := args[1]

	subscriptions[channel] = struct{}{}

	response := fmt.Sprintf(
		"*3\r\n$9\r\nsubscribe\r\n$%d\r\n%s\r\n:%d\r\n",
		len(channel),
		channel,
		len(subscriptions),
	)

	conn.Write([]byte(response))
}
