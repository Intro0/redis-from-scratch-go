package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {

	storage := &Storage{
		data: make(map[string]Value),
	}

	port := flag.Int("port", 6379, "Port to listen on")
	flag.Parse()

	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go handleConnection(conn, storage)
	}
}

func handlePing(conn net.Conn) {
	conn.Write([]byte("+PONG\r\n"))
}

func handleEcho(conn net.Conn, parts []string) {
	input := parts[4]
	response := fmt.Sprintf("$%d\r\n%s\r\n", len(input), input)
	conn.Write([]byte(response))
}

func handleSet(conn net.Conn, parts []string, storage *Storage) {
	expiry := time.Time{}
	if len(parts) > 9 {
		switch strings.ToUpper(parts[8]) {
		case "PX":
			ms, err := strconv.Atoi(parts[10])
			if err != nil {
				fmt.Println("Error with PX: ", err.Error())
			}
			expiry = time.Now().Add(time.Duration(ms) * time.Millisecond)
		case "EX":
			s, err := strconv.Atoi(parts[10])
			if err != nil {
				fmt.Println("Error with EX: ", err.Error())
			}
			expiry = time.Now().Add(time.Duration(s) * time.Second)
		default:
			fmt.Println("invalid syntax")
		}
	}
	key := parts[4]
	value := parts[6]
	storage.Set(key, StringEntry{value: value, expiry: expiry})
	conn.Write([]byte("+OK\r\n"))
}

func handleGet(conn net.Conn, parts []string, storage *Storage) {
	key := parts[4]
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

func handleType(conn net.Conn, parts []string, storage *Storage) {
	key := parts[4]
	val, ok := storage.Get(key)
	if !ok {
		fmt.Println("key not found")
		conn.Write([]byte("+none\r\n"))
		return
	}
	conn.Write([]byte("+" + val.Type() + "\r\n"))
}

func handleInfo(conn net.Conn) {
	conn.Write([]byte("$11\r\nrole:master\r\n"))
}
