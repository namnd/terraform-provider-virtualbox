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

STATE_DIR="$(dirname "$0")/state"
mkdir -p "$STATE_DIR"

vm_state_path() {
	echo "$STATE_DIR/$1"
}

vm_get_cpus() {
	path="$(vm_state_path "$1").cpus"
	if [ -f "$path" ]; then
		cat "$path"
	else
		echo 1
	fi
}

vm_get_memory() {
	path="$(vm_state_path "$1").memory"
	if [ -f "$path" ]; then
		cat "$path"
	else
		echo 1024
	fi
}

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
	vm_key="$id"
	case "$id" in
	uuid-for-*)
		vm_key="${id#uuid-for-}"
		;;
	esac
	echo "name=\"vm-$vm_key\""
	echo "UUID=\"uuid-for-$vm_key\""
	echo "cpus=$(vm_get_cpus "$id")"
	echo "memory=$(vm_get_memory "$id")"
	exit 0
	;;
modifyvm)
	id="$2"
	shift 2
	while [ $# -gt 0 ]; do
		case "$1" in
		--name)
			echo "$2" > "$(vm_state_path "$id").name"
			shift 2
			;;
		--cpus)
			echo "$2" > "$(vm_state_path "$id").cpus"
			shift 2
			;;
		--memory)
			echo "$2" > "$(vm_state_path "$id").memory"
			shift 2
			;;
		*)
			shift
			;;
		esac
	done
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
		if vm.Name != "vm-test-vm" || vm.UUID != "uuid-for-test-vm" {
			t.Fatalf("CreateVM() = %+v, want name=vm-test-vm uuid=uuid-for-test-vm", vm)
		}
		if vm.CPUs != 1 || vm.Memory != 1024 {
			t.Fatalf("CreateVM() cpus/memory = %d/%d, want 1/1024", vm.CPUs, vm.Memory)
		}
	})

	t.Run("success with cpus and memory", func(t *testing.T) {
		t.Parallel()

		vm, err := client.CreateVM(ctx, "custom-vm", CreateVMOptions{
			OSType: "Linux_64",
			CPUs:   4,
			Memory: 2048,
		})
		if err != nil {
			t.Fatalf("CreateVM() error: %v", err)
		}
		if vm.CPUs != 4 || vm.Memory != 2048 {
			t.Fatalf("CreateVM() cpus/memory = %d/%d, want 4/2048", vm.CPUs, vm.Memory)
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
		if vm.CPUs != 1 || vm.Memory != 1024 {
			t.Fatalf("GetVM() cpus/memory = %d/%d, want 1/1024", vm.CPUs, vm.Memory)
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

	t.Run("renames vm", func(t *testing.T) {
		t.Parallel()

		name := "renamed-vm"
		vm, err := client.UpdateVM(ctx, "test-vm", UpdateVMOptions{Name: &name})
		if err != nil {
			t.Fatalf("UpdateVM() error: %v", err)
		}
		if vm.Name != "vm-test-vm" || vm.UUID != "uuid-for-test-vm" {
			t.Fatalf("UpdateVM() = %+v, want name=vm-test-vm uuid=uuid-for-test-vm", vm)
		}
	})

	t.Run("updates cpus and memory", func(t *testing.T) {
		t.Parallel()

		cpus := 2
		memory := 4096
		vm, err := client.UpdateVM(ctx, "test-vm", UpdateVMOptions{
			CPUs:   &cpus,
			Memory: &memory,
		})
		if err != nil {
			t.Fatalf("UpdateVM() error: %v", err)
		}
		if vm.CPUs != 2 || vm.Memory != 4096 {
			t.Fatalf("UpdateVM() cpus/memory = %d/%d, want 2/4096", vm.CPUs, vm.Memory)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		t.Parallel()

		name := "renamed-vm"
		_, err := client.UpdateVM(ctx, "", UpdateVMOptions{Name: &name})
		if err == nil || !strings.Contains(err.Error(), "id must not be empty") {
			t.Fatalf("UpdateVM() error = %v, want empty id validation error", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()

		name := "  "
		_, err := client.UpdateVM(ctx, "test-vm", UpdateVMOptions{Name: &name})
		if err == nil || !strings.Contains(err.Error(), "name must not be empty") {
			t.Fatalf("UpdateVM() error = %v, want empty name validation error", err)
		}
	})

	t.Run("invalid cpus", func(t *testing.T) {
		t.Parallel()

		cpus := 0
		_, err := client.UpdateVM(ctx, "test-vm", UpdateVMOptions{CPUs: &cpus})
		if err == nil || !strings.Contains(err.Error(), "CPUs must be at least 1") {
			t.Fatalf("UpdateVM() error = %v, want cpus validation error", err)
		}
	})

	t.Run("invalid memory", func(t *testing.T) {
		t.Parallel()

		memory := 2
		_, err := client.UpdateVM(ctx, "test-vm", UpdateVMOptions{Memory: &memory})
		if err == nil || !strings.Contains(err.Error(), "memory must be at least 4 MB") {
			t.Fatalf("UpdateVM() error = %v, want memory validation error", err)
		}
	})

	t.Run("no changes", func(t *testing.T) {
		t.Parallel()

		_, err := client.UpdateVM(ctx, "test-vm", UpdateVMOptions{})
		if err == nil || !strings.Contains(err.Error(), "at least one VM setting must be provided") {
			t.Fatalf("UpdateVM() error = %v, want no changes validation error", err)
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
