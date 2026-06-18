// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrVMAlreadyExists is returned when creating a VM that already exists.
	ErrVMAlreadyExists = errors.New("virtual machine already exists")

	// ErrVMNotFound is returned when a VM cannot be found.
	ErrVMNotFound = errors.New("virtual machine not found")
)

// CommandError is returned when VBoxManage exits with a non-zero status.
type CommandError struct {
	Command string
	Args    []string
	Stderr  string
	Err     error
}

func (e *CommandError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	if msg == "" {
		msg = "VBoxManage command failed"
	}
	return fmt.Sprintf("VBoxManage %s: %s", strings.Join(e.Args, " "), msg)
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

func classifyVMError(stderr string) error {
	msg := strings.ToLower(stderr)
	switch {
	case strings.Contains(msg, "already exists"):
		return ErrVMAlreadyExists
	case strings.Contains(msg, "could not find a registered machine"):
		return ErrVMNotFound
	default:
		return nil
	}
}
