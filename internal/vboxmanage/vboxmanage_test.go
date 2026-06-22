// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const fakeVBoxManageScript = `#!/bin/sh
set -eu

case "$1" in
createvm)
	shift
	name=""
	while [ $# -gt 0 ]; do
		case "$1" in
		--name)
			name="$2"
			shift 2
			;;
		--register)
			shift
			;;
		--ostype|--basefolder|--groups)
			shift 2
			;;
		*)
			shift
			;;
		esac
	done
	if [ "$name" = "exists" ]; then
		echo "VBoxManage: error: Machine 'exists' already exists" >&2
		exit 1
	fi
	echo "Virtual machine '$name' is created and registered."
	echo "UUID: uuid-for-$name"
	exit 0
	;;
showvminfo)
	id="$2"
	if [ "$id" = "missing" ]; then
		echo "VBoxManage: error: Could not find a registered machine named 'missing'" >&2
		exit 1
	fi
	echo "name=\"vm-$id\""
	echo "UUID=\"uuid-for-$id\""
	exit 0
	;;
modifyvm)
	id="$2"
	new_name=""
	shift 2
	while [ $# -gt 0 ]; do
		case "$1" in
		--name)
			new_name="$2"
			shift 2
			;;
		*)
			shift
			;;
		esac
	done
	echo "name=\"$new_name\""
	echo "UUID=\"uuid-for-$id\""
	exit 0
	;;
controlvm)
	exit 0
	;;
unregistervm)
	id="$2"
	if [ "$id" = "missing" ]; then
		echo "VBoxManage: error: Could not find a registered machine named 'missing'" >&2
		exit 1
	fi
	exit 0
	;;
*)
	echo "unknown command: $1" >&2
	exit 1
	;;
esac
`

func TestCreateVM(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := newTestClient(t, fakeVBoxManageScript)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		vm, err := client.CreateVM(ctx, "test-vm", CreateVMOptions{OSType: "Linux_64"})
		if err != nil {
			t.Fatalf("CreateVM() error: %v", err)
		}
		if vm.Name != "test-vm" || vm.UUID != "uuid-for-test-vm" {
			t.Fatalf("CreateVM() = %+v, want name=test-vm uuid=uuid-for-test-vm", vm)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateVM(ctx, "  ", CreateVMOptions{})
		if err == nil || !strings.Contains(err.Error(), "name must not be empty") {
			t.Fatalf("CreateVM() error = %v, want empty name validation error", err)
		}
	})

	t.Run("already exists", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateVM(ctx, "exists", CreateVMOptions{})
		if !errors.Is(err, ErrVMAlreadyExists) {
			t.Fatalf("CreateVM() error = %v, want ErrVMAlreadyExists", err)
		}
	})
}

func TestGetVM(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := newTestClient(t, fakeVBoxManageScript)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		vm, err := client.GetVM(ctx, "test-vm")
		if err != nil {
			t.Fatalf("GetVM() error: %v", err)
		}
		if vm.Name != "vm-test-vm" || vm.UUID != "uuid-for-test-vm" {
			t.Fatalf("GetVM() = %+v, want name=vm-test-vm uuid=uuid-for-test-vm", vm)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetVM(ctx, "")
		if err == nil || !strings.Contains(err.Error(), "id must not be empty") {
			t.Fatalf("GetVM() error = %v, want empty id validation error", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetVM(ctx, "missing")
		if !errors.Is(err, ErrVMNotFound) {
			t.Fatalf("GetVM() error = %v, want ErrVMNotFound", err)
		}
	})
}

func TestUpdateVM(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := newTestClient(t, fakeVBoxManageScript)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		vm, err := client.UpdateVM(ctx, "test-vm", UpdateVMOptions{Name: "renamed-vm"})
		if err != nil {
			t.Fatalf("UpdateVM() error: %v", err)
		}
		if vm.Name != "vm-test-vm" || vm.UUID != "uuid-for-test-vm" {
			t.Fatalf("UpdateVM() = %+v, want name=vm-test-vm uuid=uuid-for-test-vm", vm)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		t.Parallel()

		_, err := client.UpdateVM(ctx, "", UpdateVMOptions{Name: "renamed-vm"})
		if err == nil || !strings.Contains(err.Error(), "id must not be empty") {
			t.Fatalf("UpdateVM() error = %v, want empty id validation error", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()

		_, err := client.UpdateVM(ctx, "test-vm", UpdateVMOptions{Name: "  "})
		if err == nil || !strings.Contains(err.Error(), "name must not be empty") {
			t.Fatalf("UpdateVM() error = %v, want empty name validation error", err)
		}
	})
}

func TestDeleteVM(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := newTestClient(t, fakeVBoxManageScript)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		if err := client.DeleteVM(ctx, "test-vm"); err != nil {
			t.Fatalf("DeleteVM() error: %v", err)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		t.Parallel()

		err := client.DeleteVM(ctx, "")
		if err == nil || !strings.Contains(err.Error(), "id must not be empty") {
			t.Fatalf("DeleteVM() error = %v, want empty id validation error", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		err := client.DeleteVM(ctx, "missing")
		if !errors.Is(err, ErrVMNotFound) {
			t.Fatalf("DeleteVM() error = %v, want ErrVMNotFound", err)
		}
	})
}
