package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// basic ping command, respong to PING with PONG
func handlePing(conn net.Conn, subscribed bool) {
	if subscribed {
		conn.Write([]byte("*2\r\n$4\r\npong\r\n$0\r\n\r\n"))
		return
	}
	conn.Write([]byte("+PONG\r\n"))
}

// basic echo command, echos back input as RESP string
func handleEcho(conn net.Conn, args []string) {
	input := args[1]
	response := fmt.Sprintf("$%d\r\n%s\r\n", len(input), input)
	conn.Write([]byte(response))
}

func handleSet(conn net.Conn, args []string, storage *Storage, aof *AOF) {
	key, entry, err := parseSet(args)
	if err != nil {
		fmt.Println("Error applying SET:", err)
		conn.Write([]byte("-ERR invalid SET command\r\n"))
		return
	}

	// saves valid command before acknowledging successful write
	if aof != nil {
		if err := aof.appendCommand(args); err != nil {
			fmt.Println("Error writing AOF:", err)
			conn.Write([]byte("-ERR failed to write AOF\r\n"))
			return
		}
	}

	storage.Set(key, entry)
	conn.Write([]byte("+OK\r\n"))
}

// validates SET args and creates string entry with optional expiry
func parseSet(args []string) (string, StringEntry, error) {
	if len(args) < 3 {
		return "", StringEntry{}, fmt.Errorf("SET requires a key and value")
	}

	expiry := time.Time{}
	if len(args) > 3 {
		if len(args) != 5 {
			return "", StringEntry{}, fmt.Errorf("invalid SET expiry arguments")
		}

		switch strings.ToUpper(args[3]) {
		case "PX":
			ms, err := strconv.Atoi(args[4])
			if err != nil {
				return "", StringEntry{}, fmt.Errorf("invalid PX expiry: %w", err)
			}
			expiry = time.Now().Add(time.Duration(ms) * time.Millisecond)
		case "EX":
			s, err := strconv.Atoi(args[4])
			if err != nil {
				return "", StringEntry{}, fmt.Errorf("invalid EX expiry: %w", err)
			}
			expiry = time.Now().Add(time.Duration(s) * time.Second)
		default:
			return "", StringEntry{}, fmt.Errorf("unsupported SET option: %s", args[3])
		}
	}

	return args[1], StringEntry{
		value:  args[2],
		expiry: expiry,
	}, nil
}

// stores parsed SET command in memory without writing AOF or response
func applySet(args []string, storage *Storage) error {
	key, entry, err := parseSet(args)
	if err != nil {
		return err
	}

	storage.Set(key, entry)
	return nil
}

// gets value if not expired, only works w/ StringEntry, Streams has XRANGE and XREAD
func handleGet(conn net.Conn, args []string, storage *Storage) {
	key := args[1]
	val, ok := storage.Get(key)
	if !ok {
		fmt.Println("value not found")
		conn.Write([]byte("$-1\r\n"))
		return
	}
	input, ok := val.(StringEntry)
	if !ok {
		conn.Write([]byte("$-1\r\n"))
		return
	}
	if !input.expiry.IsZero() && time.Now().After(input.expiry) {
		fmt.Println("value expired")
		conn.Write([]byte("$-1\r\n"))
		return
	}
	response := fmt.Sprintf("$%d\r\n%s\r\n", len(input.value), input.value)
	conn.Write([]byte(response))
}

// returns type for a key, none if missing
func handleType(conn net.Conn, args []string, storage *Storage) {
	key := args[1]
	val, ok := storage.Get(key)
	if !ok {
		fmt.Println("key not found")
		conn.Write([]byte("+none\r\n"))
		return
	}
	conn.Write([]byte("+" + val.Type() + "\r\n"))
}

// returns metadata
func handleInfo(conn net.Conn) {
	conn.Write([]byte("$11\r\nrole:master\r\n"))
}
