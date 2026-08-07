package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// reads one Redis command in RESP format, returns clean array of arguments
func readCommand(reader *bufio.Reader) ([]string, error) {
	// read first line to get number of arguments, e.g. "*3"
	header, err := readRESPLine(reader)
	if err != nil {
		return nil, err
	}

	// only accepts array requests, which is how Redis clients send commands
	if len(header) < 2 || header[0] != '*' {
		return nil, fmt.Errorf("expected RESP array header, got %q", header)
	}

	// convert argument count from string to int
	count, err := strconv.Atoi(header[1:])
	if err != nil || count < 0 {
		return nil, fmt.Errorf("invalid RESP array length: %q", header)
	}

	// create array with space for all arguments
	args := make([]string, 0, count)

	// read each command argument
	for range count {
		// read byte length header for current argument, e.g. "$4"
		bulkHeader, err := readRESPLine(reader)
		if err != nil {
			return nil, err
		}

		// only accepts bulk strings, which is how command arguments are sent
		if len(bulkHeader) < 2 || bulkHeader[0] != '$' {
			return nil, fmt.Errorf("expected bulk-string header, got %q", bulkHeader)
		}

		// convert byte length from string to int
		length, err := strconv.Atoi(bulkHeader[1:])
		if err != nil || length < 0 {
			return nil, fmt.Errorf("invalid bulk-string length: %q", bulkHeader)
		}

		// read exact number of bytes, even if TCP splits one command across reads
		value := make([]byte, length)
		// ReadFull calls reader.read until value is filled or an err is found (thats why it works w/ TCP)
		if _, err := io.ReadFull(reader, value); err != nil {
			return nil, err
		}

		// read and validate CRLF after the argument value
		terminator := make([]byte, 2)
		if _, err := io.ReadFull(reader, terminator); err != nil {
			return nil, err
		}
		if string(terminator) != "\r\n" {
			return nil, fmt.Errorf("bulk string missing CRLF terminator")
		}

		// add parsed argument to array
		args = append(args, string(value))
	}
	return args, nil
}

// reads one RESP header line and removes CRLF, e.g. "*3\r\n" becomes "*3"
func readRESPLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	if !strings.HasSuffix(line, "\r\n") {
		return "", fmt.Errorf("RESP line missing CRLF terminator")
	}

	return strings.TrimSuffix(line, "\r\n"), nil
}
