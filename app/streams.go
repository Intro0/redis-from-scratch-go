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
	val, ok := storage.Get(key)
	stream := Stream{}
	if ok {
		stream = val.(Stream)
	}

	entryID, id := resolveXAddID(args[2], stream)
	if err := validateXAddID(entryID, id, stream); err != nil {
		conn.Write(encodeError(err.Error()))
		return
	}

	entry := StreamEntry{id: entryID, values: parseStreamValues(args[3:])}
	stream.entries = append(stream.entries, entry)
	storage.Set(key, stream)
	conn.Write(encodeBulkString(entryID))
}

// resolves wildcard parts of XADD ID using the last stream entry
func resolveXAddID(value string, stream Stream) (string, StreamID) {
	id, sequenceWildcard, millisecondsWildcard := parseXAddID(value)
	if millisecondsWildcard {
		id.milliseconds = int(time.Now().UnixMilli())
	}

	if !sequenceWildcard {
		return value, id
	}

	id.sequence = 0
	if lastID, ok := lastStreamID(stream); ok && lastID.milliseconds == id.milliseconds {
		id.sequence = lastID.sequence + 1
	}
	if id.milliseconds == 0 && id.sequence == 0 {
		id.sequence = 1
	}

	return id.String(), id
}

// returns last stream ID when stream has entries
func lastStreamID(stream Stream) (StreamID, bool) {
	if len(stream.entries) == 0 {
		return StreamID{}, false
	}

	lastEntry := stream.entries[len(stream.entries)-1]
	id, _, _ := parseXAddID(lastEntry.id)
	return id, true
}

// validates XADD ID is greater than zero and last stream ID
func validateXAddID(entryID string, id StreamID, stream Stream) error {
	if entryID == "0-0" {
		return fmt.Errorf("ERR The ID specified in XADD must be greater than 0-0")
	}

	if lastID, ok := lastStreamID(stream); ok && id.compare(lastID) <= 0 {
		return fmt.Errorf("ERR The ID specified in XADD is equal or smaller than the target stream top item")
	}

	return nil
}

// converts XADD field-value arguments into entry values
func parseStreamValues(args []string) map[string]string {
	values := make(map[string]string)
	for i := 0; i+1 < len(args); i += 2 {
		values[args[i]] = args[i+1]
	}
	return values
}

func handleXRange(conn net.Conn, args []string, storage *Storage) {
	key := args[1]
	val, ok := storage.Get(key)
	if !ok {
		fmt.Println("key not found")
		conn.Write(encodeSimpleString("none"))
		return
	}

	stream := val.(Stream)
	start := parseRangeID(args[2], false)
	end := parseRangeID(args[3], true)
	entries := streamEntriesInRange(stream, start, end)
	conn.Write(encodeStreamEntries(entries))
}

// returns entries within inclusive stream ID range
func streamEntriesInRange(stream Stream, start StreamID, end StreamID) []StreamEntry {
	var entries []StreamEntry

	for _, entry := range stream.entries {
		entryID, _, _ := parseXAddID(entry.id)
		if entryID.compare(start) >= 0 && entryID.compare(end) <= 0 {
			entries = append(entries, entry)
		}
	}

	return entries
}

// encodes stream entries as nested RESP arrays
func encodeStreamEntries(entries []StreamEntry) []byte {
	var response strings.Builder

	fmt.Fprintf(&response, "*%d\r\n", len(entries))
	for _, entry := range entries {
		fmt.Fprint(&response, "*2\r\n")
		response.Write(encodeBulkString(entry.id))
		fmt.Fprintf(&response, "*%d\r\n", len(entry.values)*2)
		for field, value := range entry.values {
			response.Write(encodeBulkString(field))
			response.Write(encodeBulkString(value))
		}
	}

	return []byte(response.String())
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
