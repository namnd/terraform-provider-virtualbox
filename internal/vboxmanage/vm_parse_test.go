// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCreateVMOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		vmName  string
		stdout  string
		want    *VM
		wantErr string
	}{
		{
			name:   "parses uuid",
			vmName: "test-vm",
			stdout: "Virtual machine 'test-vm' is created and registered.\nUUID: abc-def-123\n",
			want: &VM{
				Name: "test-vm",
				UUID: "abc-def-123",
			},
		},
		{
			name:    "missing uuid",
			vmName:  "test-vm",
			stdout:  "Virtual machine 'test-vm' is created and registered.\n",
			wantErr: "UUID was not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseCreateVMOutput(tt.vmName, tt.stdout)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Name != tt.want.Name || got.UUID != tt.want.UUID {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func vmEqual(got, want *VM) bool {
	if got.Name != want.Name ||
		got.UUID != want.UUID ||
		got.CPUs != want.CPUs ||
		got.Memory != want.Memory ||
		len(got.NetworkAdapters) != len(want.NetworkAdapters) ||
		len(got.StorageControllers) != len(want.StorageControllers) {
		return false
	}
	for i := range got.NetworkAdapters {
		if got.NetworkAdapters[i] != want.NetworkAdapters[i] {
			return false
		}
	}
	for i := range got.StorageControllers {
		if got.StorageControllers[i] != want.StorageControllers[i] {
			return false
		}
	}
	return true
}

func TestParseShowVMInfoOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		stdout  string
		want    *VM
		wantErr string
	}{
		{
			name: "parses name and uuid",
			stdout: `name="test-vm"
UUID="abc-def-123"
`,
			want: &VM{
				Name: "test-vm",
				UUID: "abc-def-123",
			},
		},
		{
			name: "parses cpus and memory",
			stdout: `name="test-vm"
UUID="abc-def-123"
cpus=4
memory=2048
`,
			want: &VM{
				Name:   "test-vm",
				UUID:   "abc-def-123",
				CPUs:   4,
				Memory: 2048,
			},
		},
		{
			name:    "missing name",
			stdout:  `UUID="abc-def-123"`,
			wantErr: "name or UUID was not found",
		},
		{
			name:    "missing uuid",
			stdout:  `name="test-vm"`,
			wantErr: "name or UUID was not found",
		},
		{
			name: "parses network adapters",
			stdout: `name="test-vm"
UUID="abc-def-123"
nic1="nat"
macaddress1="080027000001"
nic2="bridged"
bridgeadapter2="eth0"
macaddress2="080027EEA5E7"
nic3="none"
`,
			want: &VM{
				Name: "test-vm",
				UUID: "abc-def-123",
				NetworkAdapters: []NetworkAdapter{
					{
						Type:            NetworkTypeNAT,
						PromiscuousMode: PromiscuousModeDeny,
						MACAddress:      "08:00:27:00:00:01",
					},
					{
						Type:            NetworkTypeBridged,
						HostInterface:   "eth0",
						PromiscuousMode: PromiscuousModeDeny,
						MACAddress:      "08:00:27:EE:A5:E7",
					},
				},
			},
		},
		{
			name: "parses storage controllers",
			stdout: `name="test-vm"
UUID="abc-def-123"
storagecontrollername0="IDE Controller"
storagecontrollertype0="PIIX4"
storagecontrollerbootable0="on"
storagecontrollername1="SATA Controller"
storagecontrollertype1="IntelAHCI"
storagecontrollerportcount1="2"
storagecontrollerbootable1="off"
`,
			want: &VM{
				Name: "test-vm",
				UUID: "abc-def-123",
				StorageControllers: []StorageController{
					{
						Name:       "IDE Controller",
						Type:       StorageBusIDE,
						Controller: StorageChipPIIX4,
						Bootable:   StorageBootableOn,
					},
					{
						Name:       "SATA Controller",
						Type:       StorageBusSATA,
						Controller: StorageChipIntelAHCI,
						Bootable:   StorageBootableOff,
						PortCount:  2,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseShowVMInfoOutput(tt.stdout)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !vmEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseNetworkAdapters(t *testing.T) {
	t.Parallel()

	stdout := `nic1="nat"
macaddress1="080027000001"
nic2="bridged"
bridgeadapter2="wlan0"
macaddress2="080027EEA5E7"
nic3="none"
nic4=""
`
	got := parseNetworkAdapters(stdout)
	want := []NetworkAdapter{
		{
			Type:            NetworkTypeNAT,
			PromiscuousMode: PromiscuousModeDeny,
			MACAddress:      "08:00:27:00:00:01",
		},
		{
			Type:            NetworkTypeBridged,
			HostInterface:   "wlan0",
			PromiscuousMode: PromiscuousModeDeny,
			MACAddress:      "08:00:27:EE:A5:E7",
		},
	}
	if len(got) != len(want) {
		t.Fatalf("parseNetworkAdapters() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseNetworkAdapters()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseStorageControllers(t *testing.T) {
	t.Parallel()

	stdout := `storagecontrollername0="IDE Controller"
storagecontrollertype0="PIIX4"
storagecontrollerbootable0="on"
storagecontrollername1="SATA Controller"
storagecontrollertype1="AHCI"
storagecontrollerportcount1="4"
storagecontrollerbootable1="off"
`
	got := parseStorageControllers(stdout)
	want := []StorageController{
		{
			Name:       "IDE Controller",
			Type:       StorageBusIDE,
			Controller: StorageChipPIIX4,
			Bootable:   StorageBootableOn,
		},
		{
			Name:       "SATA Controller",
			Type:       StorageBusSATA,
			Controller: StorageChipIntelAHCI,
			Bootable:   StorageBootableOff,
			PortCount:  4,
		},
	}
	if len(got) != len(want) {
		t.Fatalf("parseStorageControllers() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseStorageControllers()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseStorageControllerHostIOCache(t *testing.T) {
	t.Parallel()

	cfgFile := filepath.Join(t.TempDir(), "test.vbox")
	xml := `<?xml version="1.0"?>
<VirtualBox xmlns="http://www.virtualbox.org/">
  <Machine>
    <Hardware>
      <StorageControllers>
        <StorageController name="IDE Controller" type="PIIX4" PortCount="2" useHostIOCache="true" Bootable="true"/>
        <StorageController name="SATA Controller" type="IntelAHCI" PortCount="4" useHostIOCache="false" Bootable="false"/>
      </StorageControllers>
    </Hardware>
  </Machine>
</VirtualBox>`
	if err := os.WriteFile(cfgFile, []byte(xml), 0o644); err != nil {
		t.Fatalf("failed to write cfg file: %v", err)
	}

	got, err := parseStorageControllerHostIOCache(cfgFile)
	if err != nil {
		t.Fatalf("parseStorageControllerHostIOCache() error: %v", err)
	}
	want := map[string]string{
		"IDE Controller":  StorageHostIOCacheOn,
		"SATA Controller": StorageHostIOCacheOff,
	}
	for name, cache := range want {
		if got[name] != cache {
			t.Fatalf("parseStorageControllerHostIOCache()[%q] = %q, want %q", name, got[name], cache)
		}
	}
}

func TestApplyStorageControllerHostIOCache(t *testing.T) {
	t.Parallel()

	cfgFile := filepath.Join(t.TempDir(), "test.vbox")
	xml := `<?xml version="1.0"?>
<VirtualBox xmlns="http://www.virtualbox.org/">
  <Machine>
    <Hardware>
      <StorageControllers>
        <StorageController name="IDE Controller" type="PIIX4" PortCount="2" useHostIOCache="true" Bootable="true"/>
      </StorageControllers>
    </Hardware>
  </Machine>
</VirtualBox>`
	if err := os.WriteFile(cfgFile, []byte(xml), 0o644); err != nil {
		t.Fatalf("failed to write cfg file: %v", err)
	}

	vm := &VM{
		StorageControllers: []StorageController{
			{
				Name:       "IDE Controller",
				Type:       StorageBusIDE,
				Controller: StorageChipPIIX4,
			},
		},
	}
	applyStorageControllerHostIOCache(vm, cfgFile)

	if vm.StorageControllers[0].HostIOCache != StorageHostIOCacheOn {
		t.Fatalf("StorageControllers[0].HostIOCache = %q, want %q", vm.StorageControllers[0].HostIOCache, StorageHostIOCacheOn)
	}
}

func TestApplyPromiscuousModes(t *testing.T) {
	t.Parallel()

	vm := &VM{
		NetworkAdapters: []NetworkAdapter{
			{Type: NetworkTypeNAT, PromiscuousMode: PromiscuousModeDeny},
			{
				Type:            NetworkTypeBridged,
				HostInterface:   "eth0",
				PromiscuousMode: PromiscuousModeDeny,
			},
		},
	}
	stdout := `NIC 1: ... Promisc Policy: allow-vms, ...
NIC 2: ... Promisc Policy: allow-all, ...
`
	applyPromiscuousModes(vm, stdout)

	if vm.NetworkAdapters[0].PromiscuousMode != PromiscuousModeAllowVMs {
		t.Fatalf("NetworkAdapters[0].PromiscuousMode = %q, want %q", vm.NetworkAdapters[0].PromiscuousMode, PromiscuousModeAllowVMs)
	}
	if vm.NetworkAdapters[1].PromiscuousMode != PromiscuousModeAllowAll {
		t.Fatalf("NetworkAdapters[1].PromiscuousMode = %q, want %q", vm.NetworkAdapters[1].PromiscuousMode, PromiscuousModeAllowAll)
	}
}
