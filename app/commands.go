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

// store a string val (not streams), with an option for time expiry (PX for ms, EX for s)
func handleSet(conn net.Conn, args []string, storage *Storage) {
	expiry := time.Time{}
	if len(args) > 3 {
		switch strings.ToUpper(args[3]) {
		case "PX":
			ms, err := strconv.Atoi(args[4])
			if err != nil {
				fmt.Println("Error with PX: ", err.Error())
			}
			expiry = time.Now().Add(time.Duration(ms) * time.Millisecond)
		case "EX":
			s, err := strconv.Atoi(args[4])
			if err != nil {
				fmt.Println("Error with EX: ", err.Error())
			}
			expiry = time.Now().Add(time.Duration(s) * time.Second)
		default:
			fmt.Println("invalid syntax")
		}
	}
	key := args[1]
	value := args[2]
	storage.Set(key, StringEntry{value: value, expiry: expiry})
	conn.Write([]byte("+OK\r\n"))
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
