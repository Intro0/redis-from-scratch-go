package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeAOFCreatesDefaultFiles(t *testing.T) {
	config := testAOFConfig(t.TempDir())

	aof, err := initializeAOF(config)
	if err != nil {
		t.Fatalf("initializeAOF returned an error: %v", err)
	}
	t.Cleanup(func() { aof.file.Close() })

	aofDir := filepath.Join(config.dir, config.appendDirName)
	defaultAOFName := config.appendFileName + ".1.incr.aof"
	manifestFile := filepath.Join(aofDir, config.appendFileName+".manifest")

	if _, err := os.Stat(filepath.Join(aofDir, defaultAOFName)); err != nil {
		t.Fatalf("default AOF file was not created: %v", err)
	}

	manifest, err := os.ReadFile(manifestFile)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	wantManifest := "file " + defaultAOFName + " seq 1 type i\n"
	if got := string(manifest); got != wantManifest {
		t.Errorf("manifest = %q, want %q", got, wantManifest)
	}

	wantPath := filepath.Join(aofDir, defaultAOFName)
	if aof.path != wantPath {
		t.Errorf("AOF path = %q, want %q", aof.path, wantPath)
	}
}

func TestInitializeAOFUsesExistingManifest(t *testing.T) {
	config := testAOFConfig(t.TempDir())
	aofDir := filepath.Join(config.dir, config.appendDirName)
	if err := os.MkdirAll(aofDir, 0755); err != nil {
		t.Fatalf("create AOF directory: %v", err)
	}

	activeAOFName := "existing.1.incr.aof"
	activeAOFFile := filepath.Join(aofDir, activeAOFName)
	if err := os.WriteFile(activeAOFFile, nil, 0644); err != nil {
		t.Fatalf("create active AOF file: %v", err)
	}

	manifestFile := filepath.Join(aofDir, config.appendFileName+".manifest")
	manifest := "file " + activeAOFName + " seq 1 type i\n"
	if err := os.WriteFile(manifestFile, []byte(manifest), 0644); err != nil {
		t.Fatalf("create manifest: %v", err)
	}

	aof, err := initializeAOF(config)
	if err != nil {
		t.Fatalf("initializeAOF returned an error: %v", err)
	}
	t.Cleanup(func() { aof.file.Close() })

	if aof.path != activeAOFFile {
		t.Errorf("AOF path = %q, want %q", aof.path, activeAOFFile)
	}
}

// creates enabled AOF config for temporary test directory
func testAOFConfig(dir string) *Config {
	return &Config{
		dir:            dir,
		appendOnly:     "yes",
		appendDirName:  "appendonlydir",
		appendFileName: "appendonly.aof",
		appendFsync:    "always",
	}
}
