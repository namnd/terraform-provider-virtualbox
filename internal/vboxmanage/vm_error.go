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

func isRetryableCommandError(stderr string, err error) bool {
	if classifyVMError(stderr) != nil {
		return false
	}

	msg := strings.ToLower(stderr)
	if strings.Contains(msg, "already locked") ||
		strings.Contains(msg, "while it is locked") ||
		strings.Contains(msg, "vbox_e_invalid_object_state") ||
		strings.Contains(msg, "the object is not ready") ||
		strings.Contains(msg, "object functionality is limited") ||
		strings.Contains(msg, "failed to create the virtualbox object") ||
		strings.Contains(msg, "ns_error_factory_not_registered") ||
		strings.Contains(msg, "com server is not running") ||
		strings.Contains(msg, "failed to start") {
		return true
	}

	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "segmentation fault") ||
		strings.Contains(errMsg, "signal:")
}
