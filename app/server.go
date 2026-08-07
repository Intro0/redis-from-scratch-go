package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
)

// handles commands from each client
func handleConnection(conn net.Conn, storage *Storage) {
	// buffered reader keeps unread bytes for the next RESP command
	reader := bufio.NewReader(conn)

	// channels subscribed to by this specific client
	subscriptions := make(map[string]struct{})
	subscribed := false

	for {
		args, err := readCommand(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("Error reading from connection: ", err.Error())
			break
		}
		if len(args) == 0 {
			conn.Write([]byte("-ERR empty command\r\n"))
			continue
		}

		// if subscribed, we are in subscribed mode so dont allow other cmds
		command := strings.ToLower(args[0])
		if subscribed && command != "subscribe" && command != "ping" {
			response := fmt.Sprintf(
				"-ERR Can't execute '%s' in subscribed mode\r\n",
				command,
			)
			conn.Write([]byte(response))
			continue
		}

		switch command {
		case "ping":
			handlePing(conn)
		case "echo":
			handleEcho(conn, args)
		case "set":
			handleSet(conn, args, storage)
		case "get":
			handleGet(conn, args, storage)
		case "type":
			handleType(conn, args, storage)
		case "xadd":
			handleXAdd(conn, args, storage)
		case "xrange":
			handleXRange(conn, args, storage)
		case "xread":
			handleXRead(conn, args, storage)
		case "info":
			handleInfo(conn)
		case "subscribe":
			handleSubscribe(conn, args, subscriptions)
			subscribed = true
		default:
			fmt.Println("Unknown Syntax")
		}
	}
}
