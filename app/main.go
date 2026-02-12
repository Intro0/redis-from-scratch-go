package main

import (
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"flag"
)

// Value interface allows storage to hold different Redis data types (strings, streams)
type Value interface {
	Type() string
}

type StringEntry struct {
		value string
		expiry time.Time // zero value means no expiration
	}

func (e StringEntry) Type() string { return "string" }

type StreamEntry struct {
	id string
	values map[string]string
}

type Stream struct {
	entries []StreamEntry
}

// Storage wraps data map with mutex for concurrent client access
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
		go handleConnection(conn,storage) // goroutine per client for concurrent connections
	}
}

func handleConnection(conn net.Conn,storage *Storage) {
	for {
		buf:=make([]byte, 1024)
		n,err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				break // client disconnected gracefully
			}
			fmt.Println("Error reading from connection: ", err.Error())
			break
		}
		parts := strings.Split(string(buf[:n]),"\r\n") // RESP protocol splits on \r\n
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
			default:
				fmt.Println("Unknown Syntax")
		}
	}
}

// parseEntryID parses stream entry IDs like "1526985054069-0" or wildcards like "*" or "123-*"
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

// parseRangeID parses XRANGE boundaries: "-" means start of stream, "+" means end of stream
func parseRangeID(id string, isEnd bool) (ms int, seq int) {
	if id == "-" {
		ms = 0
		seq = 0
		return
	}
	if id == "+" {
		ms = math.MaxInt
		seq = math.MaxInt
		return
	}
	if strings.Contains(id, "-") {
		parts := strings.Split(id, "-")
		ms, _ = strconv.Atoi(parts[0])
		seq, _ = strconv.Atoi(parts[1])
		return
	}
	ms, _ = strconv.Atoi(id)
	if isEnd {
		seq = math.MaxInt // include all sequences when used as end boundary
	} else {
		seq = 0
	}
	return
}

func getStreamEntries(keys []string, IDs []string, storage *Storage) (allResults [][]StreamEntry, hasResult bool) {
	for i,key := range keys {
		ID := IDs[i]
		val, ok := storage.Get(key)
		if !ok {
			continue
		}
		stream := val.(Stream)
		startMS, startSeq := parseRangeID(ID, false)
		var results []StreamEntry
		for _, entry := range stream.entries {
			entryMS, entrySeq := parseRangeID(entry.id, false)
			// XREAD is exclusive (entries strictly greater than ID), unlike XRANGE which is inclusive
			if (entryMS > startMS || (entryMS == startMS && entrySeq > startSeq)) {
				results = append(results,entry)
			}
		}
		allResults = append(allResults, results)
	}
	for _, results := range allResults {
	    if len(results) > 0 {
	        hasResult = true
	        break
	    }
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
	// Auto-generate ms from current time if "*" wildcard
	if msIsWildcard {
		ms = int(time.Now().UnixMilli())
	}
	// Auto-generate seq: increment from last entry if same ms, else start at 0 (or 1 for 0-*)
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
			seq = 1 // 0-0 is reserved, minimum auto-generated ID is 0-1
		}
		id = fmt.Sprintf("%d-%d", ms, seq)
	}
	// Stream IDs must be monotonically increasing
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
	// Parse field-value pairs: RESP interleaves lengths, so step by 4 (len, key, len, val)
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
	key := parts[4]
	startID := parts[6]
	endID := parts[8]
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
		entryMS, entrySeq, _, _ := parseEntryID(entry.id)
		// XRANGE is inclusive on both ends
		if (entryMS > startMS || (entryMS == startMS && entrySeq >= startSeq)) &&
		   (entryMS < endMS || (entryMS == endMS && entrySeq <= endSeq)) {
			results = append(results,entry)
		}
	}
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

func handleXRead(conn net.Conn, parts []string, storage *Storage) {
	blockTimeout := -1 // -1 means non-blocking (no BLOCK flag)
	startIndex := 6

	if strings.ToUpper(parts[4]) == "BLOCK" {
		blockTimeout,_ = strconv.Atoi(parts[6]) // 0 means block indefinitely
		startIndex = 10                          // BLOCK + ms shifts args forward
	}
	// RESP format interleaves length prefixes with values, so skip every other element
	var args []string
	for i:=startIndex; i < len(parts); i += 2 {
		args = append(args, parts[i])
	}
	// XREAD args are: key1 key2 ... keyN id1 id2 ... idN (keys first, then IDs)
	numStreams := len(args) / 2
	keys := args[:numStreams]
	IDs := args[numStreams:]
	// Resolve "$" to the current last entry ID so the blocking loop only returns future entries
	for i, id := range(IDs) {
		if id == "$" {
			val, ok := storage.Get(keys[i])
			if ok {
				stream := val.(Stream)
				if len(stream.entries) > 0 {
					IDs[i] = stream.entries[len(stream.entries)-1].id
				} else {
					IDs[i] = "0-0"
				}
			} else {
				IDs[i] = "0-0"
			}
		}
	}
	allResults, hasResults := getStreamEntries(keys,IDs,storage)

	// Poll for new entries until timeout or data arrives; BLOCK 0 waits indefinitely
	if !hasResults && blockTimeout >= 0 {
		deadline := time.Now().Add(time.Duration(blockTimeout) * time.Millisecond)
		if blockTimeout == 0 {
			deadline = time.Now().Add(time.Hour * 24 * 365 * 100) // effectively infinite
		}
		for time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
			allResults, hasResults = getStreamEntries(keys,IDs,storage)
			if hasResults {
				break
			}
		}
	}

	if !hasResults {
		conn.Write([]byte("*-1\r\n"))
		return
	}

	var response strings.Builder
	fmt.Fprintf(&response, "*%d\r\n", len(keys))
	for i, key := range keys {
		results := allResults[i]
		fmt.Fprintf(&response, "*2\r\n")
		fmt.Fprintf(&response, "$%d\r\n%s\r\n", len(key), key)
		fmt.Fprintf(&response, "*%d\r\n", len(results))
		for _, entry := range results {
			fmt.Fprintf(&response, "*2\r\n$%d\r\n%s\r\n*%d\r\n", len(entry.id), entry.id, len(entry.values)*2)
			for k, v := range entry.values {
				fmt.Fprintf(&response, "$%d\r\n%s\r\n$%d\r\n%s\r\n", len(k), k, len(v), v)
			}
		}
	}
	conn.Write([]byte(response.String()))
}
