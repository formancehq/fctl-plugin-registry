package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "registry.yaml")
	if err := os.WriteFile(file, []byte("schemaVersion: 2\nplugins: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--file", file, "--offline"}, {"--file", file}} {
		var output bytes.Buffer
		if err := run(args, &output); err != nil {
			t.Fatal(err)
		}
		if output.String() != "Registry validation passed\n" {
			t.Fatal(output.String())
		}
	}
	for _, args := range [][]string{{"--file", filepath.Join(dir, "missing")}, {"--unknown"}, {"extra"}} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatal("accepted invalid command")
		}
	}
	if err := os.WriteFile(file, []byte("schemaVersion: 1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--file", file}, &bytes.Buffer{}); err == nil {
		t.Fatal("accepted malformed index")
	}
}
