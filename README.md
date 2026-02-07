# Redis Server Implementation in Go

A Redis server built from scratch in Go as part of the [CodeCrafters "Build Your Own Redis" Challenge](https://codecrafters.io/challenges/redis). This project implements core Redis functionality including the RESP protocol, key-value storage with expiration, concurrent client handling, and Redis Streams.

## Features Implemented

### Core Redis Commands

| Command | Description |
|---------|-------------|
| `PING` | Returns PONG - used for connection testing |
| `ECHO` | Echoes back the input message |
| `SET` | Stores a key-value pair with optional expiration (`PX` for milliseconds, `EX` for seconds) |
| `GET` | Retrieves the value for a given key (returns nil for expired/missing keys) |
| `TYPE` | Returns the data type of the value stored at a key (`string`, `stream`, or `none`) |

### Redis Streams

| Command | Description |
|---------|-------------|
| `XADD` | Appends entries to a stream with explicit or auto-generated entry IDs |
| `XRANGE` | Queries a range of entries from a stream (inclusive, supports `-` and `+`) |
| `XREAD` | Reads entries from one or more streams starting after given IDs (exclusive) |

Stream implementation includes:
- Entry ID validation (must be greater than `0-0`)
- Monotonically increasing ID enforcement (new entries must have IDs greater than the last entry)
- Partially auto-generated IDs (e.g., `1526985054069-*`)
- Fully auto-generated IDs (e.g., `*`)
- Multiple key-value pairs per stream entry
- Range queries with special `-` (start) and `+` (end) characters
- Multi-stream reads with XREAD

## Technical Implementation

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      TCP Server (:6379)                     │
├─────────────────────────────────────────────────────────────┤
│                   Concurrent Connection Handler             │
│                    (goroutines per client)                  │
├─────────────────────────────────────────────────────────────┤
│                      RESP Protocol Parser                   │
├──────────────────────┬──────────────────────────────────────┤
│    String Storage    │           Stream Storage             │
│  (with TTL support)  │    (ordered entries with IDs)        │
└──────────────────────┴──────────────────────────────────────┘
```

### Key Components

- **TCP Server**: Listens on Redis default port 6379
- **Concurrent Client Handling**: Each client connection is handled in a separate goroutine, allowing multiple simultaneous clients
- **RESP Protocol**: Parses Redis Serialization Protocol for command parsing and response formatting
- **Type System**: Polymorphic value storage using Go interfaces (`StringEntry`, `Stream`)
- **TTL Support**: Key expiration using Go's `time.Time` with millisecond and second precision

### Data Structures

```go
// String values with optional expiration
type StringEntry struct {
    value  string
    expiry time.Time
}

// Stream entries with ID and key-value pairs
type StreamEntry struct {
    id     string
    values map[string]string
}

// Stream containing ordered entries
type Stream struct {
    entries []StreamEntry
}
```

## Completed Challenge Stages

### Base Challenges (7/7)
- [x] Bind to a port - TCP server on port 6379
- [x] Respond to PING - RESP protocol implementation
- [x] Respond to multiple PINGs - Persistent connection handling
- [x] Handle concurrent clients - Goroutine-based concurrency
- [x] Implement ECHO command - Input parsing and echo response
- [x] Implement SET & GET commands - Key-value storage
- [x] Expiry - TTL with PX/EX options

### Streams Extension (11/14)
- [x] The TYPE command - Data type introspection
- [x] Create a stream - XADD command implementation
- [x] Validating entry IDs - ID ordering enforcement
- [x] Partially auto-generated IDs - Wildcard sequence numbers
- [x] Fully auto-generated IDs - Full wildcard IDs with timestamps
- [x] Query entries from stream - XRANGE command implementation
- [x] Query with `-` - Start from beginning of stream
- [x] Query with `+` - End at latest entry
- [x] Query single stream using XREAD - Exclusive read from single stream
- [x] Query multiple streams using XREAD - Read from multiple streams in one command
- [x] Blocking reads - XREAD with BLOCK timeout support
- [x] Blocking reads without timeout - XREAD with BLOCK 0 for indefinite blocking
- [ ] Blocking reads using `$`

## Running the Server

```bash
# Start the Redis server
./your_program.sh

# Or run directly with Go
go run app/main.go
```

## Example Usage

```bash
# Connect with redis-cli
redis-cli

# Basic commands
> PING
PONG

> SET mykey "Hello World"
OK

> GET mykey
"Hello World"

> SET tempkey "expires soon" PX 5000
OK

# Streams
> XADD mystream 1-0 field1 value1
"1-0"

> TYPE mystream
stream
```

## Project Structure

```
.
├── app/
│   └── main.go      # Complete Redis server implementation
├── your_program.sh  # Server startup script
└── README.md
```

## Skills Demonstrated

- **Network Programming**: TCP server implementation with concurrent connection handling
- **Protocol Implementation**: Redis Serialization Protocol (RESP) parsing and response formatting
- **Concurrency**: Goroutine-based multi-client support
- **Data Structures**: In-memory key-value store with polymorphic value types
- **Time-based Logic**: TTL implementation with expiration checking

---

Built as part of the [CodeCrafters](https://codecrafters.io) challenge series.
