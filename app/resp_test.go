package main

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
)

func TestReadCommand(t *testing.T) {
	// RESP request for: SET name Kenny
	input := "*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$5\r\nKenny\r\n"
	reader := bufio.NewReader(strings.NewReader(input))

	args, err := readCommand(reader)
	if err != nil {
		t.Fatalf("readCommand returned an error: %v", err)
	}

	want := []string{"SET", "name", "Kenny"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("readCommand returned %v, want %v", args, want)
	}
}

func TestReadCommandReadsOneCommandAtATime(t *testing.T) {
	// Two RESP PING requests sent together over one TCP connection.
	input := "*1\r\n$4\r\nPING\r\n*1\r\n$4\r\nPING\r\n"
	reader := bufio.NewReader(strings.NewReader(input))

	first, err := readCommand(reader)
	if err != nil {
		t.Fatalf("first readCommand returned an error: %v", err)
	}

	second, err := readCommand(reader)
	if err != nil {
		t.Fatalf("second readCommand returned an error: %v", err)
	}

	want := []string{"PING"}
	if !reflect.DeepEqual(first, want) {
		t.Errorf("first command returned %v, want %v", first, want)
	}
	if !reflect.DeepEqual(second, want) {
		t.Errorf("second command returned %v, want %v", second, want)
	}
}
