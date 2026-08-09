package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// creates AOF directory, incremental file, and manifest at startup
func initializeAOF(config *Config) error {
	if strings.ToLower(config.appendOnly) != "yes" {
		return nil
	}

	aofDir := filepath.Join(config.dir, config.appendDirName)

	if err := os.MkdirAll(aofDir, 0755); err != nil {
		return fmt.Errorf("create AOF directory: %w", err)
	}

	aofFile := filepath.Join(
		aofDir,
		config.appendFileName+".1.incr.aof",
	)

	file, err := os.OpenFile(
		aofFile,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return fmt.Errorf("create AOF file: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close AOF file: %w", err)
	}

	manifestFile := filepath.Join(
		aofDir,
		config.appendFileName+".manifest",
	)

	manifestContents := fmt.Sprintf(
		"file %s.1.incr.aof seq 1 type i\n",
		config.appendFileName,
	)

	if err := os.WriteFile(manifestFile, []byte(manifestContents), 0644); err != nil {
		return fmt.Errorf("create AOF manifest: %w", err)
	}

	return nil
}
