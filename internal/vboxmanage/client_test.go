// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func newTestClient(t *testing.T, script string) *Client {
	t.Helper()

	path := filepath.Join(t.TempDir(), "vboxmanage")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake VBoxManage script: %v", err)
	}

	return &Client{binary: path}
}

func TestClientVersion(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "7.1.12"
  exit 0
fi
exit 1
`)

	got, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("Version() error: %v", err)
	}
	if got != "7.1.12" {
		t.Fatalf("Version() = %q, want %q", got, "7.1.12")
	}
}

func TestClientRunWithOutputFailure(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, `#!/bin/sh
echo "command failed" >&2
exit 1
`)

	_, stderr, err := client.RunWithOutput(context.Background(), "createvm")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	cmdErr, ok := err.(*CommandError)
	if !ok {
		t.Fatalf("expected *CommandError, got %T", err)
	}
	if stderr != "command failed\n" {
		t.Fatalf("stderr = %q, want %q", stderr, "command failed\n")
	}
	if cmdErr.Args[0] != "createvm" {
		t.Fatalf("Args = %v, want createvm command", cmdErr.Args)
	}
}
