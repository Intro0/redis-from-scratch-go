package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"sync"
	"math"
)

type Value interface {
	Type() string
}

type StringEntry struct {
		value string
		expiry time.Time
	}

func (e StringEntry) Type() string { return "string" }

type StreamEntry struct {
	id string
	values map[string]string
}

type Stream struct {
	entries []StreamEntry
}

// Storage struct for Mutex Use
type Storage struct {
	data map[string]Value
	mu sync.Mutex
}

func (s *Storage) Get(key string) (Value, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *Storage) Set(key string, val Value) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
}

func (s Stream) Type() string { return "stream" }

func main() {

	storage := &Storage{
		data : make(map[string]Value),
	}

	// tcp server
	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}

	// main loop
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		// handle multiple clients thru concurrency
		go handleConnection(conn,storage)
	}
}

func handleConnection(conn net.Conn,storage *Storage) {
	for {
		buf:=make([]byte, 1024)
		n,err := conn.Read(buf)
		if err != nil {
			// check if client left so we dont need to print error
			if err == io.EOF {
				break
			}
			fmt.Println("Error reading from connection: ", err.Error())
			break
		}
		// parses arguments from input
		parts := strings.Split(string(buf[:n]),"\r\n")
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
			default:
				fmt.Println("Unknown Syntax")
		}
	}
}

func parseEntryID(id string) (ms int, seq int, seqIsWildcard bool, msIsWildcard bool) {
	if id == "*" {
		seqIsWildcard = true
		msIsWildcard = true
		return
	}
	parts := strings.Split(id, "-")
	ms, _ = strconv.Atoi(parts[0])
	if parts[1] == "*" {
		seqIsWildcard = true
	} else {
		seq, _ = strconv.Atoi(parts[1])
	}
	return
}

func parseRangeID(id string, isEnd bool) (ms int, seq int) {
	if strings.Contains(id, "-") {
		parts := strings.Split(id, "-")
		ms, _ = strconv.Atoi(parts[0])
		seq, _ = strconv.Atoi(parts[1])
		return
	}
	ms, _ = strconv.Atoi(id)
	if isEnd {
		seq = math.MaxInt
	} else {
		seq = 0
	}
	return
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

func handleXAdd(conn net.Conn, parts []string, storage *Storage) {
	key := parts[4]
	id := parts[6]
	if id == "0-0" {
		conn.Write([]byte("-ERR The ID specified in XADD must be greater than 0-0\r\n"))
		return
	}
	ms, seq, seqIsWildcard, msIsWildcard := parseEntryID(id)
	val, ok := storage.Get(key)
	if msIsWildcard {
		ms = int(time.Now().UnixMilli())
	}
	if seqIsWildcard {
		seq = 0
		if ok {
			stream := val.(Stream)
			if len(stream.entries) > 0 {
				lastEntry := stream.entries[len(stream.entries)-1]
				lastMs, lastSeq, _, _ := parseEntryID(lastEntry.id)
				if lastMs == ms {
					seq = lastSeq + 1
				}
			}
		}
		if ms == 0 && seq == 0 {
			seq = 1
		}
		id = fmt.Sprintf("%d-%d", ms, seq)
	}
	if ok {
		stream := val.(Stream)
		if len(stream.entries) > 0 {
			lastEntry := stream.entries[len(stream.entries)-1]
			lastMs, lastSeq, _, _ := parseEntryID(lastEntry.id)
			if ms < lastMs || (ms == lastMs && seq <= lastSeq) {
				conn.Write([]byte("-ERR The ID specified in XADD is equal or smaller than the target stream top item\r\n"))
				return
			}
		}
	}
	values := make(map[string]string)
	for i := 8; i+2 < len(parts); i += 4 {
		values[parts[i]] = parts[i+2]
	}
	entry := StreamEntry{id: id, values: values}
	if !ok {
		storage.Set(key, Stream{entries: []StreamEntry{entry}})
	} else {
		stream := val.(Stream)
		stream.entries = append(stream.entries, entry)
		storage.Set(key, stream)
	}
	response := fmt.Sprintf("$%d\r\n%s\r\n", len(id), id)
	conn.Write([]byte(response))
}

func handleXRange(conn net.Conn, parts []string, storage *Storage) {
	// first parse the range
	key := parts[4]
	startID := parts[6]
	endID := parts[8]
	// then retrieve the stream from storage
	val, ok := storage.Get(key)
	if !ok {
		fmt.Println("key not found")
		conn.Write([]byte("+none\r\n"))
		return
	}
	stream := val.(Stream)
	startMS, startSeq := parseRangeID(startID, false)
	endMS, endSeq := parseRangeID(endID, true)
	var results []StreamEntry
	for _, entry := range stream.entries {
		// check if entry.id is within startID and endID
		entryMS, entrySeq, _, _ := parseEntryID(entry.id)
		if (entryMS > startMS || (entryMS == startMS && entrySeq >= startSeq)) &&
		   (entryMS < endMS || (entryMS == endMS && entrySeq <= endSeq)) {
			// entry is within range, add to results
			results = append(results,entry)
		}
	}
	// finally format and send the response
	var response strings.Builder
	fmt.Fprintf(&response, "*%d\r\n", len(results))
	for _, entry := range results {
		fmt.Fprintf(&response, "*2\r\n$%d\r\n%s\r\n*%d\r\n", len(entry.id), entry.id, len(entry.values)*2)
		for k, v := range entry.values {
			fmt.Fprintf(&response, "$%d\r\n%s\r\n$%d\r\n%s\r\n", len(k), k, len(v), v)
		}
	}
	conn.Write([]byte(response.String()))
}
