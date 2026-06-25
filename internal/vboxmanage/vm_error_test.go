// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassifyVMError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stderr string
		want   error
	}{
		{
			name:   "already exists",
			stderr: "VBoxManage: error: Machine 'test' already exists",
			want:   ErrVMAlreadyExists,
		},
		{
			name:   "not found",
			stderr: "VBoxManage: error: Could not find a registered machine named 'missing'",
			want:   ErrVMNotFound,
		},
		{
			name:   "unknown error",
			stderr: "VBoxManage: error: something else went wrong",
			want:   nil,
		},
		{
			name:   "empty stderr",
			stderr: "",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyVMError(tt.stderr)
			if !errors.Is(got, tt.want) {
				t.Fatalf("classifyVMError(%q) = %v, want %v", tt.stderr, got, tt.want)
			}
		})
	}
}

func TestIsRetryableCommandError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stderr string
		err    error
		want   bool
	}{
		{
			name:   "already locked",
			stderr: "VBoxManage: error: The machine is already locked for a session",
			want:   true,
		},
		{
			name:   "object not ready",
			stderr: "VBoxManage: error: The object is not ready\nDetails: code E_ACCESSDENIED (0x80070005), component SessionMachine",
			want:   true,
		},
		{
			name:   "functionality limited",
			stderr: "VBoxManage: error: The object functionality is limited",
			want:   true,
		},
		{
			name:   "com server unavailable",
			stderr: "VBoxManage: error: Failed to create the VirtualBox object!\nVBoxManage: error: Code NS_ERROR_FACTORY_NOT_REGISTERED (0x80040154)",
			want:   true,
		},
		{
			name: "segmentation fault",
			err:  fmt.Errorf("signal: segmentation fault (core dumped)"),
			want: true,
		},
		{
			name:   "vm not found",
			stderr: "VBoxManage: error: Could not find a registered machine named 'missing'",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isRetryableCommandError(tt.stderr, tt.err); got != tt.want {
				t.Fatalf("isRetryableCommandError(stderr=%q, err=%v) = %v, want %v", tt.stderr, tt.err, got, tt.want)
			}
		})
	}
}

func TestCommandErrorError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *CommandError
		want string
	}{
		{
			name: "stderr message",
			err: &CommandError{
				Args:   []string{"createvm", "--name", "test"},
				Stderr: "machine already exists",
			},
			want: "VBoxManage createvm --name test: machine already exists",
		},
		{
			name: "fallback to wrapped error",
			err: &CommandError{
				Args: []string{"showvminfo", "test"},
				Err:  fmt.Errorf("exit status 1"),
			},
			want: "VBoxManage showvminfo test: exit status 1",
		},
		{
			name: "generic failure",
			err: &CommandError{
				Args: []string{"modifyvm", "test"},
			},
			want: "VBoxManage modifyvm test: VBoxManage command failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}
