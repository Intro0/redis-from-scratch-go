package main

import (
	"io"
	"net"
	"testing"
)

func TestServerHandlesPing(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	server := newTestServer()
	go server.handleConnection(serverConn)

	writeCommand(t, clientConn, "*1\r\n$4\r\nPING\r\n")
	assertResponse(t, clientConn, "+PONG\r\n")
}

func TestServerRoutesSetAndGet(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	server := newTestServer()
	go server.handleConnection(serverConn)

	writeCommand(t, clientConn, "*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$5\r\nKenny\r\n")
	assertResponse(t, clientConn, "+OK\r\n")

	writeCommand(t, clientConn, "*2\r\n$3\r\nGET\r\n$4\r\nname\r\n")
	assertResponse(t, clientConn, "$5\r\nKenny\r\n")
}

// creates server dependencies needed for connection tests
func newTestServer() *Server {
	return &Server{
		storage: &Storage{
			data: make(map[string]Value),
		},
		pubsub: newPubSub(),
		config: &Config{},
	}
}

// sends one RESP command through in-memory client connection
func writeCommand(t *testing.T, conn net.Conn, command string) {
	t.Helper()

	if _, err := conn.Write([]byte(command)); err != nil {
		t.Fatalf("write command: %v", err)
	}
}

// reads exact expected RESP response from in-memory client connection
func assertResponse(t *testing.T, conn net.Conn, want string) {
	t.Helper()

	response := make([]byte, len(want))
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatalf("read response: %v", err)
	}

	if got := string(response); got != want {
		t.Errorf("response = %q, want %q", got, want)
	}
}
