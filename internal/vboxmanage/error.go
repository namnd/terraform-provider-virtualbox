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

	// ErrVMLocked is returned when a VM session lock prevents configuration changes.
	ErrVMLocked = errors.New("virtual machine is locked")

	// ErrMediumAlreadyExists is returned when creating a disk that already exists.
	ErrMediumAlreadyExists = errors.New("disk medium already exists")

	// ErrMediumNotFound is returned when a disk medium cannot be found.
	ErrMediumNotFound = errors.New("disk medium not found")

	// ErrStorageControllerNotFound is returned when a storage controller cannot be found.
	ErrStorageControllerNotFound = errors.New("storage controller not found")

	// ErrStorageAttachmentNotFound is returned when a storage attachment cannot be found.
	ErrStorageAttachmentNotFound = errors.New("storage attachment not found")
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
	case strings.Contains(msg, "already locked for a session"),
		strings.Contains(msg, "lock request pending"):
		return ErrVMLocked
	default:
		return nil
	}
}

func classifyStorageError(stderr string) error {
	msg := strings.ToLower(stderr)
	switch {
	case strings.Contains(msg, "could not find a storage controller"),
		strings.Contains(msg, "could not find a controller named"):
		return ErrStorageControllerNotFound
	case strings.Contains(msg, "no storage device attached"):
		return ErrStorageAttachmentNotFound
	case strings.Contains(msg, "could not find a registered machine"):
		return ErrVMNotFound
	case strings.Contains(msg, "already locked for a session"),
		strings.Contains(msg, "lock request pending"):
		return ErrVMLocked
	default:
		return nil
	}
}

func isBenignStorageDeleteError(err error) bool {
	return errors.Is(err, ErrVMNotFound) ||
		errors.Is(err, ErrStorageControllerNotFound) ||
		errors.Is(err, ErrStorageAttachmentNotFound)
}

func isVMTransientError(err error) bool {
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		return false
	}

	msg := strings.ToLower(cmdErr.Stderr)
	return strings.Contains(msg, "object is not ready") ||
		strings.Contains(msg, "e_accessdenied") ||
		strings.Contains(msg, "already locked for a session") ||
		strings.Contains(msg, "lock request pending")
}
