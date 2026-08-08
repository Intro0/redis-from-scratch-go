# Redis server in Go

A Redis-inspired server built from scratch in Go. It speaks RESP over TCP and currently supports strings with TTLs, Streams, Pub/Sub, and early append-only-file configuration.

Built while working through the [CodeCrafters "Build Your Own Redis" challenge](https://codecrafters.io/challenges/redis). The server is organized into separate protocol, storage, command, stream, Pub/Sub, and configuration components.

## Run it

```bash
./your_program.sh
```

The server listens on port `6379` by default. Use a different port when needed:

```bash
./your_program.sh --port 6380
redis-cli -p 6380
```

## Supported commands

`PING`, `ECHO`, `SET`, `GET`, `TYPE`, `INFO`, `XADD`, `XRANGE`, `XREAD`, `SUBSCRIBE`, `UNSUBSCRIBE`, `PUBLISH`, and `CONFIG GET` for AOF settings.

```redis
# strings and expiry
SET user:1 "Kenny"
SET session:1 "temporary" EX 60
GET user:1

# streams
XADD orders * item keyboard quantity 1
XRANGE orders - +
XREAD BLOCK 5000 STREAMS orders $

# pub/sub (run the subscribe command in one redis-cli session)
SUBSCRIBE notifications

# publish from another session
PUBLISH notifications "new order received"
```

The server also accepts AOF-related startup options. At this point these configure the server and create the append-only directory when enabled; command persistence comes next.

```bash
./your_program.sh \
  --dir /tmp/redis-data \
  --appendonly yes \
  --appenddirname appendonlydir

redis-cli CONFIG GET appendonly
redis-cli CONFIG GET dir
```

## Implementation

- A TCP listener accepts clients, with one goroutine per connection.
- A buffered RESP reader handles commands even when TCP splits or combines requests.
- The in-memory store is protected with a mutex and stores multiple value types through a small Go interface.
- Streams maintain ordered IDs and support range, multi-stream, and blocking reads.
- Pub/Sub keeps a concurrent channel-to-subscriber registry and sends messages to a snapshot of subscribers.

## Project structure

```text
app/
├── main.go       # server startup and configuration
├── server.go     # client connections and command routing
├── resp.go       # RESP request parser
├── storage.go    # in-memory values and synchronization
├── commands.go   # string commands
├── streams.go    # stream commands
├── pubsub.go     # subscriptions and message delivery
└── config.go     # AOF configuration
```

## Tests

```bash
go test ./...
codecrafters test
```
