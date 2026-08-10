package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// holds active AOF file, path, sync setting, and mutex for concurrent writes
type AOF struct {
	file        *os.File
	path        string
	appendFsync string
	mu          sync.Mutex
}

// creates AOF directory, incremental file, and manifest at startup
func initializeAOF(config *Config) (*AOF, error) {
	if strings.ToLower(config.appendOnly) != "yes" {
		return nil, nil
	}

	aofDir := filepath.Join(config.dir, config.appendDirName)

	if err := os.MkdirAll(aofDir, 0755); err != nil {
		return nil, fmt.Errorf("create AOF directory: %w", err)
	}

	// creates default incremental AOF file if missing
	defaultAOFName := config.appendFileName + ".1.incr.aof"
	defaultAOFFile := filepath.Join(aofDir, defaultAOFName)

	// O_CREATE creates file if missing, O_APPEND writes at end, O_WRONLY allows writes
	// 0644 gives owner read/write access and group/others read-only access
	file, err := os.OpenFile(
		defaultAOFFile,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)

	if err != nil {
		return nil, fmt.Errorf("create default AOF file: %w", err)
	}

	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close default AOF file: %w", err)
	}

	manifestFile := filepath.Join(
		aofDir,
		config.appendFileName+".manifest",
	)

	// only creates manifest if one does not already exist
	if _, err := os.Stat(manifestFile); errors.Is(err, fs.ErrNotExist) {
		manifestContents := fmt.Sprintf(
			"file %s seq 1 type i\n",
			defaultAOFName,
		)

		if err := os.WriteFile(manifestFile, []byte(manifestContents), 0644); err != nil {
			return nil, fmt.Errorf("create AOF manifest: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("check AOF manifest: %w", err)
	}

	// reads manifest to find incremental AOF file used for writes
	activeAOFName, err := readActiveAOFName(manifestFile)
	if err != nil {
		return nil, err
	}

	activeAOFFile := filepath.Join(aofDir, activeAOFName)

	// opens manifest-selected file for future command writes
	file, err = os.OpenFile(
		activeAOFFile,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return nil, fmt.Errorf("open active AOF file: %w", err)
	}

	return &AOF{
		file:        file,
		path:        activeAOFFile,
		appendFsync: config.appendFsync,
	}, nil
}

// reads manifest to find active incremental AOF filename
func readActiveAOFName(manifestFile string) (string, error) {
	contents, err := os.ReadFile(manifestFile)
	if err != nil {
		return "", fmt.Errorf("read AOF manifest: %w", err)
	}

	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)

		if len(fields) == 6 &&
			fields[0] == "file" &&
			fields[2] == "seq" &&
			fields[4] == "type" &&
			fields[5] == "i" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("active incremental AOF file not found")
}

// reads saved RESP commands and restores them in storage
func replayAOF(aof *AOF, storage *Storage) error {
	if aof == nil {
		return nil
	}

	file, err := os.Open(aof.path)
	if err != nil {
		return fmt.Errorf("open AOF for replay: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	for {
		args, err := readCommand(reader)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read AOF command: %w", err)
		}
		if len(args) == 0 {
			return fmt.Errorf("empty AOF command")
		}

		switch strings.ToLower(args[0]) {
		case "set":
			if err := applySet(args, storage); err != nil {
				return fmt.Errorf("replay SET command: %w", err)
			}
		default:
			return fmt.Errorf("unsupported AOF command: %s", args[0])
		}
	}
}

// appends RESP command to AOF and syncs when configured
func (aof *AOF) appendCommand(args []string) error {
	// locks file so concurrent writes do not overlap
	aof.mu.Lock()
	defer aof.mu.Unlock()

	command := encodeCommand(args)

	if _, err := aof.file.Write(command); err != nil {
		return fmt.Errorf("write AOF command: %w", err)
	}

	if strings.ToLower(aof.appendFsync) == "always" {
		// forces AOF write to disk before sending client response
		if err := aof.file.Sync(); err != nil {
			return fmt.Errorf("sync AOF file: %w", err)
		}
	}
	return nil
}

// converts command args into RESP bytes for AOF
func encodeCommand(args []string) []byte {
	var response strings.Builder

	fmt.Fprintf(&response, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&response, "$%d\r\n%s\r\n", len(arg), arg)
	}
	return []byte(response.String())
}
