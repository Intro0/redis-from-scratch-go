package main

import (
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"
)

type StreamEntry struct {
	id     string
	values map[string]string
}

type Stream struct {
	entries []StreamEntry
}

func (s Stream) Type() string { return "stream" }

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
	if id == "-" {
		return 0, 0
	}
	if id == "+" {
		return math.MaxInt, math.MaxInt
	}
	if strings.Contains(id, "-") {
		parts := strings.Split(id, "-")
		ms, _ = strconv.Atoi(parts[0])
		seq, _ = strconv.Atoi(parts[1])
		return
	}
	ms, _ = strconv.Atoi(id)
	if isEnd {
		seq = math.MaxInt
	}
	return
}

func getStreamEntries(keys []string, IDs []string, storage *Storage) (allResults [][]StreamEntry, hasResult bool) {
	for i, key := range keys {
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
			if entryMS > startMS || (entryMS == startMS && entrySeq > startSeq) {
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
		if (entryMS > startMS || (entryMS == startMS && entrySeq >= startSeq)) &&
			(entryMS < endMS || (entryMS == endMS && entrySeq <= endSeq)) {
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

func handleXRead(conn net.Conn, parts []string, storage *Storage) {
	blockTimeout := -1
	startIndex := 6
	if strings.ToUpper(parts[4]) == "BLOCK" {
		blockTimeout, _ = strconv.Atoi(parts[6])
		startIndex = 10
	}
	var args []string
	for i := startIndex; i < len(parts); i += 2 {
		args = append(args, parts[i])
	}
	numStreams := len(args) / 2
	keys := args[:numStreams]
	IDs := args[numStreams:]
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
