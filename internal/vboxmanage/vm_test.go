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

storage_indices_path() {
	echo "$(vm_state_path "$1").storage_indices"
}

storage_index_path() {
	echo "$(vm_state_path "$1").storage_$2"
}

storage_next_index() {
	indices_file="$(storage_indices_path "$1")"
	if [ ! -f "$indices_file" ]; then
		echo 0
		return
	fi
	max=-1
	for idx in $(cat "$indices_file"); do
		if [ "$idx" -gt "$max" ]; then
			max="$idx"
		fi
	done
	echo $((max + 1))
}

storage_find_index_by_name() {
	id="$1"
	name="$2"
	indices_file="$(storage_indices_path "$id")"
	if [ ! -f "$indices_file" ]; then
		return 1
	fi
	for idx in $(cat "$indices_file"); do
		base="$(storage_index_path "$id" "$idx")"
		if [ -f "${base}.name" ] && [ "$(cat "${base}.name")" = "$name" ]; then
			echo "$idx"
			return 0
		fi
	done
	return 1
}

storage_add_index() {
	id="$1"
	idx="$2"
	indices_file="$(storage_indices_path "$id")"
	if [ -f "$indices_file" ]; then
		echo "$idx" >> "$indices_file"
	else
		echo "$idx" > "$indices_file"
	fi
}

storage_remove_index() {
	id="$1"
	idx="$2"
	indices_file="$(storage_indices_path "$id")"
	base="$(storage_index_path "$id" "$idx")"
	rm -f "${base}.name" "${base}.chip" "${base}.bus" "${base}.portcount" "${base}.bootable" "${base}.hostiocache"
	if [ ! -f "$indices_file" ]; then
		return
	fi
	new_file="${indices_file}.new"
	: > "$new_file"
	for i in $(cat "$indices_file"); do
		if [ "$i" != "$idx" ]; then
			echo "$i" >> "$new_file"
		fi
	done
	if [ -s "$new_file" ]; then
		mv "$new_file" "$indices_file"
	else
		rm -f "$indices_file" "$new_file"
	fi
}

storage_write_cfg_file() {
	id="$1"
	cfg="$(vm_state_path "$id").vbox"
	indices_file="$(storage_indices_path "$id")"
	if [ ! -f "$indices_file" ]; then
		rm -f "$cfg"
		return 1
	fi
	{
		echo '<?xml version="1.0"?>'
		echo '<VirtualBox xmlns="http://www.virtualbox.org/">'
		echo '  <Machine>'
		echo '    <Hardware>'
		echo '      <StorageControllers>'
		for idx in $(cat "$indices_file"); do
			base="$(storage_index_path "$id" "$idx")"
			name="$(cat "${base}.name" 2>/dev/null || echo "")"
			chip="$(cat "${base}.chip" 2>/dev/null || echo "")"
			portcount="$(cat "${base}.portcount" 2>/dev/null || echo 0)"
			bootable="$(cat "${base}.bootable" 2>/dev/null || echo off)"
			hostiocache="$(cat "${base}.hostiocache" 2>/dev/null || echo off)"
			use_host_io_cache="false"
			if [ "$hostiocache" = "on" ]; then
				use_host_io_cache="true"
			fi
			bootable_attr="false"
			if [ "$bootable" = "on" ]; then
				bootable_attr="true"
			fi
			echo "        <StorageController name=\"$name\" type=\"$chip\" PortCount=\"$portcount\" useHostIOCache=\"$use_host_io_cache\" Bootable=\"$bootable_attr\"/>"
		done
		echo '      </StorageControllers>'
		echo '    </Hardware>'
		echo '  </Machine>'
		echo '</VirtualBox>'
	} > "$cfg"
	echo "$cfg"
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
	shift 2
	machine_readable=false
	for arg in "$@"; do
		if [ "$arg" = "--machinereadable" ]; then
			machine_readable=true
		fi
	done
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
	if [ "$machine_readable" = true ]; then
		echo "name=\"vm-$vm_key\""
		echo "UUID=\"uuid-for-$vm_key\""
		echo "cpus=$(vm_get_cpus "$id")"
		echo "memory=$(vm_get_memory "$id")"
		for i in 1 2 3 4; do
			nic_path="$(vm_state_path "$id").nic$i"
			if [ -f "$nic_path" ]; then
				echo "nic$i=\"$(cat "$nic_path")\""
			fi
			bridge_path="$(vm_state_path "$id").bridgeadapter$i"
			if [ -f "$bridge_path" ]; then
				echo "bridgeadapter$i=\"$(cat "$bridge_path")\""
			fi
		done
		indices_file="$(storage_indices_path "$id")"
		if [ -f "$indices_file" ]; then
			for idx in $(cat "$indices_file"); do
				base="$(storage_index_path "$id" "$idx")"
				if [ -f "${base}.name" ]; then
					echo "storagecontrollername$idx=\"$(cat "${base}.name")\""
				fi
				if [ -f "${base}.chip" ]; then
					echo "storagecontrollertype$idx=\"$(cat "${base}.chip")\""
				fi
				if [ -f "${base}.portcount" ]; then
					echo "storagecontrollerportcount$idx=\"$(cat "${base}.portcount")\""
				fi
				if [ -f "${base}.bootable" ]; then
					echo "storagecontrollerbootable$idx=\"$(cat "${base}.bootable")\""
				fi
			done
			cfg="$(storage_write_cfg_file "$id")"
			if [ -n "$cfg" ]; then
				echo "CfgFile=\"$cfg\""
			fi
		fi
	else
		for i in 1 2 3 4; do
			nic_path="$(vm_state_path "$id").nic$i"
			if [ -f "$nic_path" ]; then
				promisc_path="$(vm_state_path "$id").nicpromisc$i"
				promisc="deny"
				if [ -f "$promisc_path" ]; then
					promisc="$(cat "$promisc_path")"
				fi
				echo "NIC $i: ... Promisc Policy: $promisc, ..."
			fi
		done
	fi
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
		--nic1|--nic2|--nic3|--nic4)
			echo "$2" > "$(vm_state_path "$id").${1#--}"
			shift 2
			;;
		--nicpromisc1|--nicpromisc2|--nicpromisc3|--nicpromisc4)
			echo "$2" > "$(vm_state_path "$id").${1#--}"
			shift 2
			;;
		--bridgeadapter1|--bridgeadapter2|--bridgeadapter3|--bridgeadapter4)
			echo "$2" > "$(vm_state_path "$id").${1#--}"
			shift 2
			;;
		*)
			shift
			;;
		esac
	done
	exit 0
	;;
storagectl)
	id="$2"
	shift 2
	name=""
	add_type=""
	remove=false
	controller=""
	bootable=""
	hostiocache=""
	portcount=""
	while [ $# -gt 0 ]; do
		case "$1" in
		--name)
			name="$2"
			shift 2
			;;
		--add)
			add_type="$2"
			shift 2
			;;
		--remove)
			remove=true
			shift
			;;
		--controller)
			controller="$2"
			shift 2
			;;
		--bootable)
			bootable="$2"
			shift 2
			;;
		--hostiocache)
			hostiocache="$2"
			shift 2
			;;
		--portcount)
			portcount="$2"
			shift 2
			;;
		*)
			shift
			;;
		esac
	done
	if [ "$remove" = true ]; then
		idx="$(storage_find_index_by_name "$id" "$name" || true)"
		if [ -n "$idx" ]; then
			storage_remove_index "$id" "$idx"
		fi
		exit 0
	fi
	idx="$(storage_find_index_by_name "$id" "$name" || true)"
	if [ -z "$idx" ]; then
		idx="$(storage_next_index "$id")"
		storage_add_index "$id" "$idx"
	fi
	base="$(storage_index_path "$id" "$idx")"
	echo "$name" > "${base}.name"
	if [ -n "$add_type" ]; then
		echo "$add_type" > "${base}.bus"
	fi
	if [ -n "$controller" ]; then
		echo "$controller" > "${base}.chip"
	fi
	if [ -n "$bootable" ]; then
		echo "$bootable" > "${base}.bootable"
	fi
	if [ -n "$hostiocache" ]; then
		echo "$hostiocache" > "${base}.hostiocache"
	fi
	if [ -n "$portcount" ]; then
		echo "$portcount" > "${base}.portcount"
	fi
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

	t.Run("success with storage controllers", func(t *testing.T) {
		t.Parallel()

		vm, err := client.CreateVM(ctx, "storage-vm", CreateVMOptions{
			OSType: "Linux_64",
			StorageControllers: []StorageController{
				{
					Name:        "IDE Controller",
					Type:        StorageBusIDE,
					Controller:  StorageChipPIIX4,
					HostIOCache: StorageHostIOCacheOn,
				},
				{
					Name:       "SATA Controller",
					Type:       StorageBusSATA,
					Controller: StorageChipIntelAHCI,
					Bootable:   StorageBootableOn,
					PortCount:  2,
				},
			},
		})
		if err != nil {
			t.Fatalf("CreateVM() error: %v", err)
		}
		if len(vm.StorageControllers) != 2 {
			t.Fatalf("CreateVM() StorageControllers len = %d, want 2", len(vm.StorageControllers))
		}
		if vm.StorageControllers[0].Type != StorageBusIDE || vm.StorageControllers[0].Controller != StorageChipPIIX4 {
			t.Fatalf("StorageControllers[0] = %+v, want IDE PIIX4", vm.StorageControllers[0])
		}
		if vm.StorageControllers[0].HostIOCache != StorageHostIOCacheOn {
			t.Fatalf("StorageControllers[0].HostIOCache = %q, want %q", vm.StorageControllers[0].HostIOCache, StorageHostIOCacheOn)
		}
		if vm.StorageControllers[1].Type != StorageBusSATA || vm.StorageControllers[1].PortCount != 2 {
			t.Fatalf("StorageControllers[1] = %+v, want SATA with port count 2", vm.StorageControllers[1])
		}
		if vm.StorageControllers[1].Bootable != StorageBootableOn {
			t.Fatalf("StorageControllers[1].Bootable = %q, want %q", vm.StorageControllers[1].Bootable, StorageBootableOn)
		}
	})

	t.Run("invalid storage controller", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateVM(ctx, "bad-storage-vm", CreateVMOptions{
			StorageControllers: []StorageController{
				{
					Name:       "IDE Controller",
					Type:       StorageBusIDE,
					Controller: StorageChipIntelAHCI,
				},
			},
		})
		if err == nil || !strings.Contains(err.Error(), `controller "IntelAHCI" is not valid for type "ide"`) {
			t.Fatalf("CreateVM() error = %v, want storage controller validation error", err)
		}
	})

	t.Run("success with network adapters", func(t *testing.T) {
		t.Parallel()

		vm, err := client.CreateVM(ctx, "net-vm", CreateVMOptions{
			NetworkAdapters: []NetworkAdapter{
				{Type: NetworkTypeNAT, PromiscuousMode: PromiscuousModeDeny},
				{
					Type:            NetworkTypeBridged,
					HostInterface:   "eth0",
					PromiscuousMode: PromiscuousModeAllowAll,
				},
			},
		})
		if err != nil {
			t.Fatalf("CreateVM() error: %v", err)
		}
		if len(vm.NetworkAdapters) != 2 {
			t.Fatalf("CreateVM() NetworkAdapters len = %d, want 2", len(vm.NetworkAdapters))
		}
		if vm.NetworkAdapters[0].Type != NetworkTypeNAT {
			t.Fatalf("NetworkAdapters[0].Type = %q, want %q", vm.NetworkAdapters[0].Type, NetworkTypeNAT)
		}
		if vm.NetworkAdapters[1].Type != NetworkTypeBridged || vm.NetworkAdapters[1].HostInterface != "eth0" {
			t.Fatalf("NetworkAdapters[1] = %+v, want bridged on eth0", vm.NetworkAdapters[1])
		}
		if vm.NetworkAdapters[1].PromiscuousMode != PromiscuousModeAllowAll {
			t.Fatalf("NetworkAdapters[1].PromiscuousMode = %q, want %q", vm.NetworkAdapters[1].PromiscuousMode, PromiscuousModeAllowAll)
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

	t.Run("returns network adapters", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateVM(ctx, "get-net-vm", CreateVMOptions{
			NetworkAdapters: []NetworkAdapter{
				{Type: NetworkTypeNAT, PromiscuousMode: PromiscuousModeAllowVMs},
			},
		})
		if err != nil {
			t.Fatalf("CreateVM() error: %v", err)
		}

		vm, err := client.GetVM(ctx, "uuid-for-get-net-vm")
		if err != nil {
			t.Fatalf("GetVM() error: %v", err)
		}
		if len(vm.NetworkAdapters) != 1 {
			t.Fatalf("GetVM() NetworkAdapters len = %d, want 1", len(vm.NetworkAdapters))
		}
		if vm.NetworkAdapters[0].Type != NetworkTypeNAT {
			t.Fatalf("NetworkAdapters[0].Type = %q, want %q", vm.NetworkAdapters[0].Type, NetworkTypeNAT)
		}
		if vm.NetworkAdapters[0].PromiscuousMode != PromiscuousModeAllowVMs {
			t.Fatalf("NetworkAdapters[0].PromiscuousMode = %q, want %q", vm.NetworkAdapters[0].PromiscuousMode, PromiscuousModeAllowVMs)
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

	t.Run("updates network adapters", func(t *testing.T) {
		t.Parallel()

		adapters := []NetworkAdapter{
			{
				Type:            NetworkTypeBridged,
				HostInterface:   "wlan0",
				PromiscuousMode: PromiscuousModeAllowAll,
			},
		}
		vm, err := client.UpdateVM(ctx, "test-vm", UpdateVMOptions{NetworkAdapters: &adapters})
		if err != nil {
			t.Fatalf("UpdateVM() error: %v", err)
		}
		if len(vm.NetworkAdapters) != 1 {
			t.Fatalf("UpdateVM() NetworkAdapters len = %d, want 1", len(vm.NetworkAdapters))
		}
		if vm.NetworkAdapters[0].Type != NetworkTypeBridged || vm.NetworkAdapters[0].HostInterface != "wlan0" {
			t.Fatalf("NetworkAdapters[0] = %+v, want bridged on wlan0", vm.NetworkAdapters[0])
		}
		if vm.NetworkAdapters[0].PromiscuousMode != PromiscuousModeAllowAll {
			t.Fatalf("NetworkAdapters[0].PromiscuousMode = %q, want %q", vm.NetworkAdapters[0].PromiscuousMode, PromiscuousModeAllowAll)
		}
	})

	t.Run("invalid network adapter", func(t *testing.T) {
		t.Parallel()

		adapters := []NetworkAdapter{{Type: NetworkTypeBridged}}
		_, err := client.UpdateVM(ctx, "test-vm", UpdateVMOptions{NetworkAdapters: &adapters})
		if err == nil || !strings.Contains(err.Error(), "host_interface is required") {
			t.Fatalf("UpdateVM() error = %v, want host_interface validation error", err)
		}
	})

	t.Run("updates storage controllers", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateVM(ctx, "update-storage-vm", CreateVMOptions{
			StorageControllers: []StorageController{
				{
					Name:       "IDE Controller",
					Type:       StorageBusIDE,
					Controller: StorageChipPIIX4,
				},
			},
		})
		if err != nil {
			t.Fatalf("CreateVM() error: %v", err)
		}

		controllers := []StorageController{
			{
				Name:        "IDE Controller",
				Type:        StorageBusIDE,
				Controller:  StorageChipPIIX4,
				HostIOCache: StorageHostIOCacheOn,
			},
			{
				Name:       "SATA Controller",
				Type:       StorageBusSATA,
				Controller: StorageChipIntelAHCI,
				Bootable:   StorageBootableOn,
				PortCount:  1,
			},
		}
		vm, err := client.UpdateVM(ctx, "uuid-for-update-storage-vm", UpdateVMOptions{StorageControllers: &controllers})
		if err != nil {
			t.Fatalf("UpdateVM() error: %v", err)
		}
		if len(vm.StorageControllers) != 2 {
			t.Fatalf("UpdateVM() StorageControllers len = %d, want 2", len(vm.StorageControllers))
		}
		if vm.StorageControllers[0].HostIOCache != StorageHostIOCacheOn {
			t.Fatalf("StorageControllers[0].HostIOCache = %q, want %q", vm.StorageControllers[0].HostIOCache, StorageHostIOCacheOn)
		}
		if vm.StorageControllers[1].Type != StorageBusSATA || vm.StorageControllers[1].PortCount != 1 {
			t.Fatalf("StorageControllers[1] = %+v, want SATA with port count 1", vm.StorageControllers[1])
		}
	})

	t.Run("invalid storage controller", func(t *testing.T) {
		t.Parallel()

		controllers := []StorageController{
			{
				Name: "Bad Controller",
				Type: "invalid",
			},
		}
		_, err := client.UpdateVM(ctx, "test-vm", UpdateVMOptions{StorageControllers: &controllers})
		if err == nil || !strings.Contains(err.Error(), "unsupported storage controller type") {
			t.Fatalf("UpdateVM() error = %v, want storage controller validation error", err)
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
