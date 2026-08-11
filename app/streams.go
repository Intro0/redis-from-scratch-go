package main

import (
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"
)

// single entry in a stream, ordered by timestamp (id)
type StreamEntry struct {
	id     string
	values map[string]string
}

// ordered collection of stream entries, under one redis key
type Stream struct {
	entries []StreamEntry
}

func (s Stream) Type() string { return "stream" }

// stores timestamp and sequence parts of a stream entry ID
type StreamID struct {
	milliseconds int
	sequence     int
}

// converts stream ID into Redis timestamp-sequence format
func (id StreamID) String() string {
	return fmt.Sprintf("%d-%d", id.milliseconds, id.sequence)
}

// compares stream IDs by timestamp first, then sequence
func (id StreamID) compare(other StreamID) int {
	if id.milliseconds < other.milliseconds {
		return -1
	}
	if id.milliseconds > other.milliseconds {
		return 1
	}
	if id.sequence < other.sequence {
		return -1
	}
	if id.sequence > other.sequence {
		return 1
	}
	return 0
}

// parses stream ID given in XADD, including wildcard parts
func parseXAddID(value string) (id StreamID, sequenceWildcard bool, millisecondsWildcard bool) {
	if value == "*" {
		sequenceWildcard = true
		millisecondsWildcard = true
		return
	}

	parts := strings.Split(value, "-")
	id.milliseconds, _ = strconv.Atoi(parts[0])
	if parts[1] == "*" {
		sequenceWildcard = true
	} else {
		id.sequence, _ = strconv.Atoi(parts[1])
	}
	return
}

// parse XRANGE into timestamp and sequence
// - and + represent start and end of a Stream
func parseRangeID(value string, isEnd bool) StreamID {
	if value == "-" {
		return StreamID{}
	}
	if value == "+" {
		return StreamID{milliseconds: math.MaxInt, sequence: math.MaxInt}
	}
	if strings.Contains(value, "-") {
		id, _, _ := parseXAddID(value)
		return id
	}

	milliseconds, _ := strconv.Atoi(value)
	if isEnd {
		return StreamID{milliseconds: milliseconds, sequence: math.MaxInt}
	}

	return StreamID{milliseconds: milliseconds}
}

// appends an entry to a Stream
func handleXAdd(conn net.Conn, args []string, storage *Storage) {
	key := args[1]
	entryID := args[2]
	if entryID == "0-0" {
		conn.Write([]byte("-ERR The ID specified in XADD must be greater than 0-0\r\n"))
		return
	}

	id, sequenceWildcard, millisecondsWildcard := parseXAddID(entryID)
	val, ok := storage.Get(key)
	if millisecondsWildcard {
		id.milliseconds = int(time.Now().UnixMilli())
	}
	if sequenceWildcard {
		id.sequence = 0
		if ok {
			stream := val.(Stream)
			if len(stream.entries) > 0 {
				lastEntry := stream.entries[len(stream.entries)-1]
				lastID, _, _ := parseXAddID(lastEntry.id)
				if lastID.milliseconds == id.milliseconds {
					id.sequence = lastID.sequence + 1
				}
			}
		}
		if id.milliseconds == 0 && id.sequence == 0 {
			id.sequence = 1
		}
		entryID = id.String()
	}

	if ok {
		stream := val.(Stream)
		if len(stream.entries) > 0 {
			lastEntry := stream.entries[len(stream.entries)-1]
			lastID, _, _ := parseXAddID(lastEntry.id)
			if id.compare(lastID) <= 0 {
				conn.Write([]byte("-ERR The ID specified in XADD is equal or smaller than the target stream top item\r\n"))
				return
			}
		}
	}
	values := make(map[string]string)
	for i := 3; i+1 < len(args); i += 2 {
		values[args[i]] = args[i+1]
	}
	entry := StreamEntry{id: entryID, values: values}
	if !ok {
		storage.Set(key, Stream{entries: []StreamEntry{entry}})
	} else {
		stream := val.(Stream)
		stream.entries = append(stream.entries, entry)
		storage.Set(key, stream)
	}
	conn.Write(encodeBulkString(entryID))
}

func handleXRange(conn net.Conn, args []string, storage *Storage) {
	key := args[1]
	startID := args[2]
	endID := args[3]
	val, ok := storage.Get(key)
	if !ok {
		fmt.Println("key not found")
		conn.Write([]byte("+none\r\n"))
		return
	}
	stream := val.(Stream)
	start := parseRangeID(startID, false)
	end := parseRangeID(endID, true)
	var results []StreamEntry
	for _, entry := range stream.entries {
		entryID, _, _ := parseXAddID(entry.id)
		if entryID.compare(start) >= 0 && entryID.compare(end) <= 0 {
			results = append(results, entry)
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

func getStreamEntries(keys []string, IDs []string, storage *Storage) (allResults [][]StreamEntry, hasResult bool) {
	for i, key := range keys {
		ID := IDs[i]
		val, ok := storage.Get(key)
		if !ok {
			continue
		}
		stream := val.(Stream)
		start := parseRangeID(ID, false)
		var results []StreamEntry
		for _, entry := range stream.entries {
			entryID := parseRangeID(entry.id, false)
			if entryID.compare(start) > 0 {
				results = append(results, entry)
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

func handleXRead(conn net.Conn, args []string, storage *Storage) {
	blockTimeout := -1
	streamsIndex := 1
	if strings.ToUpper(args[1]) == "BLOCK" {
		blockTimeout, _ = strconv.Atoi(args[2])
		streamsIndex = 3
	}
	streamArgs := args[streamsIndex+1:]
	numStreams := len(streamArgs) / 2
	keys := streamArgs[:numStreams]
	IDs := streamArgs[numStreams:]
	for i, id := range IDs {
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
	allResults, hasResults := getStreamEntries(keys, IDs, storage)
	if !hasResults && blockTimeout >= 0 {
		deadline := time.Now().Add(time.Duration(blockTimeout) * time.Millisecond)
		if blockTimeout == 0 {
			deadline = time.Now().Add(time.Hour * 24 * 365 * 100)
		}
		for time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
			allResults, hasResults = getStreamEntries(keys, IDs, storage)
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
