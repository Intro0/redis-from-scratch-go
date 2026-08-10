package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
)

// holds shared state used by all client connections
type Server struct {
	storage *Storage
	pubsub  *PubSub
	config  *Config
	aof     *AOF
}

// handles commands from each client
func (s *Server) handleConnection(conn net.Conn) {
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
		if subscribed && command != "subscribe" && command != "ping" && command != "unsubscribe" {
			response := fmt.Sprintf(
				"-ERR Can't execute '%s' in subscribed mode\r\n",
				command,
			)
			conn.Write([]byte(response))
			continue
		}

		switch command {
		case "ping":
			handlePing(conn, subscribed)
		case "echo":
			handleEcho(conn, args)
		case "set":
			handleSet(conn, args, s.storage, s.aof)
		case "get":
			handleGet(conn, args, s.storage)
		case "type":
			handleType(conn, args, s.storage)
		case "xadd":
			handleXAdd(conn, args, s.storage)
		case "xrange":
			handleXRange(conn, args, s.storage)
		case "xread":
			handleXRead(conn, args, s.storage)
		case "info":
			handleInfo(conn)
		case "subscribe":
			handleSubscribe(conn, args, subscriptions, s.pubsub)
			subscribed = true
		case "publish":
			handlePublish(conn, args, s.pubsub)
		case "unsubscribe":
			handleUnsubscribe(conn, args, subscriptions, s.pubsub)
			subscribed = len(subscriptions) > 0
		case "config":
			handleConfig(conn, args, s.config)
		default:
			fmt.Println("Unknown Syntax")
		}
	}
}
