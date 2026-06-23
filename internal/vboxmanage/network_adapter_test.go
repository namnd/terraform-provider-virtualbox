// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"strings"
	"testing"
)

func TestValidateNetworkAdapter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		adapter NetworkAdapter
		wantErr string
	}{
		{
			name:    "nat adapter",
			adapter: NetworkAdapter{Type: NetworkTypeNAT},
		},
		{
			name: "bridged adapter with host interface",
			adapter: NetworkAdapter{
				Type:          NetworkTypeBridged,
				HostInterface: "eth0",
			},
		},
		{
			name: "bridged adapter with promiscuous mode",
			adapter: NetworkAdapter{
				Type:            NetworkTypeBridged,
				HostInterface:   "wlan0",
				PromiscuousMode: PromiscuousModeAllowAll,
			},
		},
		{
			name:    "empty type",
			adapter: NetworkAdapter{},
			wantErr: "network adapter type must not be empty",
		},
		{
			name:    "unsupported type",
			adapter: NetworkAdapter{Type: "hostonly"},
			wantErr: "unsupported network adapter type",
		},
		{
			name:    "bridged without host interface",
			adapter: NetworkAdapter{Type: NetworkTypeBridged},
			wantErr: "host_interface is required for bridged network adapters",
		},
		{
			name: "invalid promiscuous mode",
			adapter: NetworkAdapter{
				Type:            NetworkTypeNAT,
				PromiscuousMode: "invalid",
			},
			wantErr: "unsupported promiscuous_mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateNetworkAdapter(tt.adapter)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateNetworkAdapter() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNormalizePromiscuousMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode string
		want string
	}{
		{mode: "", want: PromiscuousModeDeny},
		{mode: PromiscuousModeDeny, want: PromiscuousModeDeny},
		{mode: PromiscuousModeAllowVMs, want: PromiscuousModeAllowVMs},
		{mode: PromiscuousModeAllowAll, want: PromiscuousModeAllowAll},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()

			if got := NormalizePromiscuousMode(tt.mode); got != tt.want {
				t.Fatalf("NormalizePromiscuousMode(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestNetworkModifyVMArgs(t *testing.T) {
	t.Parallel()

	t.Run("single nat adapter", func(t *testing.T) {
		t.Parallel()

		args, err := networkModifyVMArgs([]NetworkAdapter{
			{Type: NetworkTypeNAT},
		})
		if err != nil {
			t.Fatalf("networkModifyVMArgs() error: %v", err)
		}
		want := []string{
			"--nic1", NetworkTypeNAT,
			"--nicpromisc1", PromiscuousModeDeny,
			"--nic2", "none",
			"--nic3", "none",
			"--nic4", "none",
		}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			t.Fatalf("networkModifyVMArgs() = %v, want %v", args, want)
		}
	})

	t.Run("bridged adapter with host interface", func(t *testing.T) {
		t.Parallel()

		args, err := networkModifyVMArgs([]NetworkAdapter{
			{
				Type:            NetworkTypeBridged,
				HostInterface:   "eth0",
				PromiscuousMode: PromiscuousModeAllowAll,
			},
		})
		if err != nil {
			t.Fatalf("networkModifyVMArgs() error: %v", err)
		}
		want := []string{
			"--nic1", NetworkTypeBridged,
			"--nicpromisc1", PromiscuousModeAllowAll,
			"--bridgeadapter1", "eth0",
			"--nic2", "none",
			"--nic3", "none",
			"--nic4", "none",
		}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			t.Fatalf("networkModifyVMArgs() = %v, want %v", args, want)
		}
	})

	t.Run("multiple adapters", func(t *testing.T) {
		t.Parallel()

		args, err := networkModifyVMArgs([]NetworkAdapter{
			{Type: NetworkTypeNAT},
			{
				Type:            NetworkTypeBridged,
				HostInterface:   "wlan0",
				PromiscuousMode: PromiscuousModeAllowVMs,
			},
		})
		if err != nil {
			t.Fatalf("networkModifyVMArgs() error: %v", err)
		}
		want := []string{
			"--nic1", NetworkTypeNAT,
			"--nicpromisc1", PromiscuousModeDeny,
			"--nic2", NetworkTypeBridged,
			"--nicpromisc2", PromiscuousModeAllowVMs,
			"--bridgeadapter2", "wlan0",
			"--nic3", "none",
			"--nic4", "none",
		}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			t.Fatalf("networkModifyVMArgs() = %v, want %v", args, want)
		}
	})

	t.Run("invalid adapter", func(t *testing.T) {
		t.Parallel()

		_, err := networkModifyVMArgs([]NetworkAdapter{
			{Type: NetworkTypeBridged},
		})
		if err == nil {
			t.Fatal("expected error for invalid adapter")
		}
		if !strings.Contains(err.Error(), "network adapter 0") {
			t.Fatalf("error = %q, want adapter index in message", err.Error())
		}
	})
}

func TestBuildModifyVMArgsNetworkAdapters(t *testing.T) {
	t.Parallel()

	adapters := []NetworkAdapter{
		{Type: NetworkTypeNAT, PromiscuousMode: PromiscuousModeDeny},
	}
	args, err := buildModifyVMArgs("uuid-123", UpdateVMOptions{NetworkAdapters: &adapters})
	if err != nil {
		t.Fatalf("buildModifyVMArgs() error: %v", err)
	}
	if args[0] != "modifyvm" || args[1] != "uuid-123" {
		t.Fatalf("buildModifyVMArgs() prefix = %v, want modifyvm uuid-123", args[:2])
	}
	if !strings.Contains(strings.Join(args, " "), "--nic1 nat") {
		t.Fatalf("buildModifyVMArgs() = %v, want nic args", args)
	}

	invalid := []NetworkAdapter{{Type: "invalid"}}
	_, err = buildModifyVMArgs("uuid-123", UpdateVMOptions{NetworkAdapters: &invalid})
	if err == nil {
		t.Fatal("expected error for invalid network adapter")
	}
}

func TestUpdateVMOptionsHasChanges(t *testing.T) {
	t.Parallel()

	adapters := []NetworkAdapter{{Type: NetworkTypeNAT}}
	opts := UpdateVMOptions{NetworkAdapters: &adapters}
	if !opts.HasChanges() {
		t.Fatal("expected HasChanges() to be true when network adapters are set")
	}

	empty := UpdateVMOptions{}
	if empty.HasChanges() {
		t.Fatal("expected HasChanges() to be false for empty options")
	}
}
