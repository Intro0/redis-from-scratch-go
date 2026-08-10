package main

import (
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
		conn.Write(encodeError("ERR unsupported CONFIG command"))
		return
	}

	option := strings.ToLower(args[2])
	value, ok := config.get(option)
	if !ok {
		conn.Write(encodeError("ERR unsupported CONFIG option"))
		return
	}

	conn.Write(encodeBulkStringArray([]string{option, value}))
}
