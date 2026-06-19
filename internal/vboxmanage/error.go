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

	// ErrMediumAlreadyExists is returned when creating a disk that already exists.
	ErrMediumAlreadyExists = errors.New("disk medium already exists")

	// ErrMediumNotFound is returned when a disk medium cannot be found.
	ErrMediumNotFound = errors.New("disk medium not found")
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

func classifyMediumError(stderr string) error {
	msg := strings.ToLower(stderr)
	switch {
	case strings.Contains(msg, "already exists"), strings.Contains(msg, "verr_already_exists"):
		return ErrMediumAlreadyExists
	case strings.Contains(msg, "could not find file for the medium"),
		strings.Contains(msg, "verr_file_not_found"),
		strings.Contains(msg, "could not find a medium"):
		return ErrMediumNotFound
	default:
		return nil
	}
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
