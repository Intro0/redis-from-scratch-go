package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

func main() {

	// init storage used by all client connections
	storage := &Storage{
		data: make(map[string]Value),
	}
	pubsub := newPubSub()

	// grab port from --port flag, 6379 default
	port := flag.Int("port", 6379, "Port to listen on")

	// gets workign directory to store config for AOF Persistence
	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Println("Failed to get working directory:", err)
		os.Exit(1)
	}

	// command line flags for AOF config
	dir := flag.String("dir", workingDir, "AOF directory")
	appendOnly := flag.String("appendonly", "no", "Enable AOF persistence")
	appendDirName := flag.String("appenddirname", "appendonlydir", "AOF subdirectory")
	appendFileName := flag.String("appendfilename", "appendonly.aof", "AOF file name")
	appendFsync := flag.String("appendfsync", "everysec", "AOF sync policy")

	flag.Parse()

	config := &Config{
		dir:            *dir,
		appendOnly:     *appendOnly,
		appendDirName:  *appendDirName,
		appendFileName: *appendFileName,
		appendFsync:    *appendFsync,
	}

	// checks for appendOnly flag, if so, creates AOF directory with rx all, rwx for owner
	if strings.ToLower(config.appendOnly) == "yes" {
		aofDir := filepath.Join(config.dir, config.appendDirName)

		if err := os.MkdirAll(aofDir, 0755); err != nil {
			fmt.Println("Failed to create AOF directory:", err)
			os.Exit(1)
		}
	}

	// listen for TCP connections on selected port and all network interfaces
	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}

	// handles multiple connections concurrently using goroutine
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go handleConnection(conn, storage, pubsub, config)
	}
}
