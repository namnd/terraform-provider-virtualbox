// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// binary is the path to the VBoxManage executable (VBoxManage on PATH).
const defaultBinary = "VBoxManage"

const commandMaxAttempts = 10

// Client is a Go wrapper around the VBoxManage CLI.
type Client struct {
	binary   string
	globalMu sync.Mutex
	vmLocks  sync.Map
}

// New creates a Client that invokes the VBoxManage CLI.
// TODO: allow to pass custom path to the binary.
func New() (VirtualBox, error) {
	c := &Client{
		binary: defaultBinary,
	}

	if _, err := exec.LookPath(c.binary); err != nil {
		return nil, fmt.Errorf("VBoxManage binary %q not found in PATH: %w", c.binary, err)
	}

	return c, nil
}

// Run executes VBoxManage with the given arguments and returns stdout.
func (c *Client) Run(ctx context.Context, args ...string) (string, error) {
	stdout, _, err := c.RunWithOutput(ctx, args...)
	return stdout, err
}

// RunWithOutput executes VBoxManage and returns stdout, stderr, and any error.
// All VBoxManage invocations are serialized globally because VirtualBox exposes
// a single COM/XPCOM service that cannot safely handle concurrent CLI access.
func (c *Client) RunWithOutput(ctx context.Context, args ...string) (stdout, stderr string, err error) {
	c.globalMu.Lock()
	defer c.globalMu.Unlock()

	var lastErr error
	for attempt := range commandMaxAttempts {
		stdout, stderr, err = c.runWithOutputOnce(ctx, args...)
		if err == nil {
			return stdout, stderr, nil
		}

		if vmErr := classifyVMError(stderr); vmErr != nil {
			return stdout, stderr, vmErr
		}

		lastErr = err
		if !isRetryableCommandError(stderr, err) || attempt == commandMaxAttempts-1 {
			return stdout, stderr, err
		}

		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(commandRetryDelay(attempt)):
		}
	}

	return stdout, stderr, lastErr
}

func (c *Client) runWithOutputOnce(ctx context.Context, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, c.binary, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if err == nil {
		return stdout, stderr, nil
	}

	return stdout, stderr, &CommandError{
		Command: c.binary,
		Args:    args,
		Stderr:  stderr,
		Err:     err,
	}
}

func commandRetryDelay(attempt int) time.Duration {
	// 500ms, 1s, 1.5s, ... gives VBoxSVC time to recover between attempts.
	return time.Duration(500*(attempt+1)) * time.Millisecond
}

// Version returns the installed VirtualBox version string.
func (c *Client) Version(ctx context.Context) (string, error) {
	stdout, err := c.Run(ctx, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}
