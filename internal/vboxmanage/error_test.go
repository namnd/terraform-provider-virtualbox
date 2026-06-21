// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"errors"
	"testing"
)

func TestClassifyStorageError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stderr string
		want   error
	}{
		{
			name:   "controller not found",
			stderr: "VBoxManage: error: Could not find a storage controller named 'IDE Controller'",
			want:   ErrStorageControllerNotFound,
		},
		{
			name:   "attachment not found",
			stderr: "VBoxManage: error: No storage device attached to device slot 0 on port 1 of controller 'IDE Controller'",
			want:   ErrStorageAttachmentNotFound,
		},
		{
			name:   "vm not found",
			stderr: "VBoxManage: error: Could not find a registered machine",
			want:   ErrVMNotFound,
		},
		{
			name:   "vm locked",
			stderr: "VBoxManage: error: The machine 'test-2' is already locked for a session (or being unlocked)",
			want:   ErrVMLocked,
		},
		{
			name:   "lock request pending",
			stderr: "VBoxManage: error: The machine 'test-2' already has a lock request pending",
			want:   ErrVMLocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyStorageError(tt.stderr)
			if !errors.Is(got, tt.want) {
				t.Fatalf("classifyStorageError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsVMTransientError(t *testing.T) {
	t.Parallel()

	if !isVMTransientError(ErrVMLocked) {
		t.Fatal("expected ErrVMLocked to be transient")
	}

	cmdErr := &CommandError{
		Stderr: "VBoxManage: error: The object is not ready\nVBoxManage: error: Details: code E_ACCESSDENIED (0x80070005)",
	}
	if !isVMTransientError(cmdErr) {
		t.Fatal("expected object is not ready error to be transient")
	}

	if isVMTransientError(errors.New("boom")) {
		t.Fatal("expected generic error to not be transient")
	}
}

func TestIsBenignStorageDeleteError(t *testing.T) {
	t.Parallel()

	if !isBenignStorageDeleteError(ErrStorageControllerNotFound) {
		t.Fatal("expected storage controller not found to be benign")
	}
	if isBenignStorageDeleteError(errors.New("boom")) {
		t.Fatal("expected generic error to not be benign")
	}
}
