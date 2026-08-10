package main

import (
	"fmt"
	"strings"
)

// encodes RESP simple string, used for responses like +OK
func encodeSimpleString(value string) []byte {
	return []byte("+" + value + "\r\n")
}

// encodes RESP error response
func encodeError(message string) []byte {
	return []byte("-" + message + "\r\n")
}

// encodes RESP bulk string
func encodeBulkString(value string) []byte {
	return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(value), value))
}

// encodes RESP null bulk string
func encodeNullBulkString() []byte {
	return []byte("$-1\r\n")
}

// encodes RESP integer
func encodeInteger(value int) []byte {
	return []byte(fmt.Sprintf(":%d\r\n", value))
}

// encodes RESP array containing only bulk strings
func encodeBulkStringArray(values []string) []byte {
	var response strings.Builder

	fmt.Fprintf(&response, "*%d\r\n", len(values))
	for _, value := range values {
		response.Write(encodeBulkString(value))
	}

	return []byte(response.String())
}
