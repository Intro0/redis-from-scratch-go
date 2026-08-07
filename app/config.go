package main

import (
	"fmt"
	"net"
	"strings"
)

// stores server config values
type Config struct {
	dir            string
	appendOnly     string
	appendDirName  string
	appendFileName string
	appendFsync    string
}

// creates new config with defaults
func newConfig(dir string) *Config {
	return &Config{
		dir:            dir,
		appendOnly:     "no",
		appendDirName:  "appendonlydir",
		appendFileName: "appendonly.aof",
		appendFsync:    "everysec",
	}
}

// gets one config value by name
func (c *Config) get(option string) (string, bool) {
	switch strings.ToLower(option) {
	case "dir":
		return c.dir, true
	case "appendonly":
		return c.appendOnly, true
	case "appenddirname":
		return c.appendDirName, true
	case "appendfilename":
		return c.appendFileName, true
	case "appendfsync":
		return c.appendFsync, true
	default:
		return "", false
	}
}

// returns a config option and its value
func handleConfig(conn net.Conn, args []string, config *Config) {
	if len(args) != 3 || strings.ToLower(args[1]) != "get" {
		conn.Write([]byte("-ERR unsupported CONFIG command\r\n"))
		return
	}

	option := strings.ToLower(args[2])
	value, ok := config.get(option)
	if !ok {
		conn.Write([]byte("-ERR unsupported CONFIG option\r\n"))
		return
	}

	response := fmt.Sprintf(
		"*2\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
		len(option),
		option,
		len(value),
		value,
	)
	conn.Write([]byte(response))
}
