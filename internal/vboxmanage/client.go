// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// binary is the path to the VBoxManage executable (VBoxManage on PATH).
const defaultBinary = "VBoxManage"

// Client is a Go wrapper around the VBoxManage CLI.
type Client struct {
	binary string
}

// Ensure Client implements VirtualBox.
var _ VirtualBox = (*Client)(nil)

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
func (c *Client) RunWithOutput(ctx context.Context, args ...string) (stdout, stderr string, err error) {
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

// Version returns the installed VirtualBox version string.
func (c *Client) Version(ctx context.Context) (string, error) {
	stdout, err := c.Run(ctx, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}
