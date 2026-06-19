// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"errors"
	"testing"
)

func TestParseVMStorageMachineReadable(t *testing.T) {
	t.Parallel()

	stdout := `name="test-vm"
UUID="00000000-0000-0000-0000-000000000001"
storagecontrollername0="IDE Controller"
storagecontrollertype0="PIIX4"
storagecontrollermaxportcount0="2"
storagecontrollerbootable0="on"
storagecontrollerhostiocache0="on"
"IDE Controller-1-0"="/path/to/metal-amd64.iso"
"IDE Controller-ImageUUID-1-0"="d3190f8e-ba95-45a4-afe5-16a81457527d"
`

	storage, err := parseVMStorageMachineReadable(stdout, "IDE Controller", 1, 0)
	if err != nil {
		t.Fatalf("parseVMStorageMachineReadable() error = %v", err)
	}

	if storage.Name != "IDE Controller" {
		t.Fatalf("Name = %q, want %q", storage.Name, "IDE Controller")
	}
	if storage.Type != StorageTypeIDE {
		t.Fatalf("Type = %q, want %q", storage.Type, StorageTypeIDE)
	}
	if storage.Controller != "PIIX4" {
		t.Fatalf("Controller = %q, want %q", storage.Controller, "PIIX4")
	}
	if storage.PortCount != 2 {
		t.Fatalf("PortCount = %d, want %d", storage.PortCount, 2)
	}
	if !storage.HostIOCache {
		t.Fatal("expected HostIOCache to be true")
	}
	if !storage.Bootable {
		t.Fatal("expected Bootable to be true")
	}
	if storage.Attachment.Medium != "/path/to/metal-amd64.iso" {
		t.Fatalf("Attachment.Medium = %q, want %q", storage.Attachment.Medium, "/path/to/metal-amd64.iso")
	}
	if storage.Attachment.Type != StorageAttachTypeDVDDrive {
		t.Fatalf("Attachment.Type = %q, want %q", storage.Attachment.Type, StorageAttachTypeDVDDrive)
	}
}

func TestParseVMStorageMachineReadableNotFound(t *testing.T) {
	t.Parallel()

	stdout := `storagecontrollername0="IDE Controller"
storagecontrollertype0="PIIX4"
IDE Controller-0-0="none"
`

	_, err := parseVMStorageMachineReadable(stdout, "IDE Controller", 1, 0)
	if !errors.Is(err, ErrVMStorageNotFound) {
		t.Fatalf("parseVMStorageMachineReadable() error = %v, want %v", err, ErrVMStorageNotFound)
	}
}

func TestParseVMStorageMachineReadableSATAController(t *testing.T) {
	t.Parallel()

	stdout := `storagecontrollername0="SATA Controller"
storagecontrollertype0="IntelAhci"
storagecontrollermaxportcount0="30"
storagecontrollerbootable0="on"
"SATA Controller-0-0"="/data/test.vdi"
`

	storage, err := parseVMStorageMachineReadable(stdout, "SATA Controller", 0, 0)
	if err != nil {
		t.Fatalf("parseVMStorageMachineReadable() error = %v", err)
	}
	if storage.Type != StorageTypeSATA {
		t.Fatalf("Type = %q, want %q", storage.Type, StorageTypeSATA)
	}
	if storage.Controller != StorageControllerIntelAHCI {
		t.Fatalf("Controller = %q, want %q", storage.Controller, StorageControllerIntelAHCI)
	}
}

func TestCanonicalStorageControllerChipset(t *testing.T) {
	t.Parallel()

	if got := CanonicalStorageControllerChipset("IntelAhci"); got != StorageControllerIntelAHCI {
		t.Fatalf("CanonicalStorageControllerChipset() = %q, want %q", got, StorageControllerIntelAHCI)
	}
}

func TestInferStorageAttachType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		medium string
		want   string
	}{
		{medium: "/path/to/os.iso", want: StorageAttachTypeDVDDrive},
		{medium: "/path/to/disk.vdi", want: StorageAttachTypeHDD},
		{medium: "/path/to/floppy.img", want: StorageAttachTypeFDD},
	}

	for _, tt := range tests {
		if got := inferStorageAttachType(tt.medium); got != tt.want {
			t.Fatalf("inferStorageAttachType(%q) = %q, want %q", tt.medium, got, tt.want)
		}
	}
}
