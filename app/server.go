package main

import (
	"fmt"
	"io"
	"net"
	"strings"
)

func handleConnection(conn net.Conn, storage *Storage) {
	for {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("Error reading from connection: ", err.Error())
			break
		}
		parts := strings.Split(string(buf[:n]), "\r\n")
		switch strings.ToLower(parts[2]) {
		case "ping":
			handlePing(conn)
		case "echo":
			handleEcho(conn, parts)
		case "set":
			handleSet(conn, parts, storage)
		case "get":
			handleGet(conn, parts, storage)
		case "type":
			handleType(conn, parts, storage)
		case "xadd":
			handleXAdd(conn, parts, storage)
		case "xrange":
			handleXRange(conn, parts, storage)
		case "xread":
			handleXRead(conn, parts, storage)
		case "info":
			handleInfo(conn)
		default:
			fmt.Println("Unknown Syntax")
		}
	}
}
