// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"errors"
	"testing"
)

func TestParseCreateVMOutput(t *testing.T) {
	t.Parallel()

	stdout := `Virtual machine 'my-vm' is created and registered.
UUID: 9f69463b-2426-49be-8ad2-cb609e20953b
Settings file: '/Users/test/VirtualBox VMs/my-vm/my-vm.vbox'
`

	vm, err := parseCreateVMOutput("my-vm", stdout)
	if err != nil {
		t.Fatalf("parseCreateVMOutput() error = %v", err)
	}

	if vm.Name != "my-vm" {
		t.Fatalf("Name = %q, want %q", vm.Name, "my-vm")
	}
	if vm.UUID != "9f69463b-2426-49be-8ad2-cb609e20953b" {
		t.Fatalf("UUID = %q, want expected UUID", vm.UUID)
	}
}

func TestParseCreateVMOutputMissingUUID(t *testing.T) {
	t.Parallel()

	_, err := parseCreateVMOutput("my-vm", "Virtual machine 'my-vm' is created and registered.")
	if err == nil {
		t.Fatal("expected error for missing UUID, got nil")
	}
}

func TestParseShowVMInfoOutput(t *testing.T) {
	t.Parallel()

	stdout := `name="my-vm"
encryption="disabled"
groups="/"
ostype="Other Linux (64-bit)"
UUID="9f69463b-2426-49be-8ad2-cb609e20953b"
memory=2048
cpus=2
nic1="nat"
nic2="bridged"
bridgeadapter2="enp0s3"
CfgFile="/Users/test/VirtualBox VMs/my-vm/my-vm.vbox"
`

	vm, err := parseShowVMInfoOutput(stdout)
	if err != nil {
		t.Fatalf("parseShowVMInfoOutput() error = %v", err)
	}

	if vm.Name != "my-vm" {
		t.Fatalf("Name = %q, want %q", vm.Name, "my-vm")
	}
	if vm.UUID != "9f69463b-2426-49be-8ad2-cb609e20953b" {
		t.Fatalf("UUID = %q, want expected UUID", vm.UUID)
	}
	if vm.OSType != "Linux_64" {
		t.Fatalf("OSType = %q, want %q", vm.OSType, "Linux_64")
	}
	if vm.Memory != 2048 {
		t.Fatalf("Memory = %d, want %d", vm.Memory, 2048)
	}
	if vm.CPUs != 2 {
		t.Fatalf("CPUs = %d, want %d", vm.CPUs, 2)
	}
	if len(vm.NetworkAdapters) != 2 {
		t.Fatalf("len(NetworkAdapters) = %d, want %d", len(vm.NetworkAdapters), 2)
	}
	if vm.NetworkAdapters[0].Type != "nat" {
		t.Fatalf("NetworkAdapters[0].Type = %q, want %q", vm.NetworkAdapters[0].Type, "nat")
	}
	if vm.NetworkAdapters[1].Type != "bridged" {
		t.Fatalf("NetworkAdapters[1].Type = %q, want %q", vm.NetworkAdapters[1].Type, "bridged")
	}
	if vm.NetworkAdapters[1].HostInterface != "enp0s3" {
		t.Fatalf("NetworkAdapters[1].HostInterface = %q, want %q", vm.NetworkAdapters[1].HostInterface, "enp0s3")
	}
}

func TestParseShowVMInfoOutputMissingName(t *testing.T) {
	t.Parallel()

	stdout := `UUID="9f69463b-2426-49be-8ad2-cb609e20953b"
CfgFile="/Users/test/VirtualBox VMs/my-vm/my-vm.vbox"
`

	_, err := parseShowVMInfoOutput(stdout)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestParseShowVMInfoOutputMissingUUID(t *testing.T) {
	t.Parallel()

	stdout := `name="my-vm"
CfgFile="/Users/test/VirtualBox VMs/my-vm/my-vm.vbox"
`

	_, err := parseShowVMInfoOutput(stdout)
	if err == nil {
		t.Fatal("expected error for missing UUID, got nil")
	}
}

func TestParsePromiscuousModes(t *testing.T) {
	t.Parallel()

	stdout := `NIC 1:                       MAC: 080027EEA5E7, Attachment: NAT, Cable connected: on, Trace: off (file: none), Type: 82540EM, Reported speed: 0 Mbps, Boot priority: 0, Promisc Policy: allow-vms, Bandwidth group: none
NIC 2:                       MAC: 08002741A4F8, Attachment: Bridged, Cable connected: on, Trace: off (file: none), Type: 82540EM, Reported speed: 0 Mbps, Boot priority: 0, Promisc Policy: allow-all, Bandwidth group: none
`

	modes := parsePromiscuousModes(stdout)
	if modes[1] != "allow-vms" {
		t.Fatalf("modes[1] = %q, want %q", modes[1], "allow-vms")
	}
	if modes[2] != "allow-all" {
		t.Fatalf("modes[2] = %q, want %q", modes[2], "allow-all")
	}
}

func TestClassifyVMError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stderr string
		want   error
	}{
		{
			name:   "already exists",
			stderr: "VBoxManage: error: Machine settings file already exists",
			want:   ErrVMAlreadyExists,
		},
		{
			name:   "not found",
			stderr: "VBoxManage: error: Could not find a registered machine named 'missing'",
			want:   ErrVMNotFound,
		},
		{
			name:   "unknown",
			stderr: "VBoxManage: error: something else",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyVMError(tt.stderr)
			if !errors.Is(got, tt.want) {
				t.Fatalf("classifyVMError() = %v, want %v", got, tt.want)
			}
		})
	}
}
