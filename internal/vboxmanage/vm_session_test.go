// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"errors"
	"testing"
)

func TestIsVMLockError(t *testing.T) {
	t.Parallel()

	if !isVMLockError(ErrVMLocked) {
		t.Fatal("expected ErrVMLocked to be a lock error")
	}

	cmdErr := &CommandError{
		Stderr: "VBoxManage: error: The machine 'test-2' already has a lock request pending",
	}
	if !isVMLockError(classifyCommandError(cmdErr.Stderr, cmdErr)) {
		t.Fatal("expected classified lock pending error")
	}

	if isVMLockError(errors.New("boom")) {
		t.Fatal("expected generic error to not be a lock error")
	}
}

func TestIsVMStartable(t *testing.T) {
	t.Parallel()

	if !isVMStartable("poweroff") {
		t.Fatal("expected powered off VM to be startable")
	}
	if !isVMStartable("saved") {
		t.Fatal("expected saved VM to be startable")
	}
	if isVMStartable("running") {
		t.Fatal("expected running VM to not be startable")
	}
}

func TestVMStateNeedsPowerOff(t *testing.T) {
	t.Parallel()

	if !vmStateNeedsPowerOff("running") {
		t.Fatal("expected running VM to need power off")
	}
	if !vmStateNeedsPowerOff("saved") {
		t.Fatal("expected saved VM to need power off")
	}
	if vmStateNeedsPowerOff("poweroff") {
		t.Fatal("expected powered off VM to not need power off")
	}
}
