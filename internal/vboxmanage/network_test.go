// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import "testing"

func TestValidateNetworkAdapter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		adapter NetworkAdapter
		wantErr bool
	}{
		{
			name:    "nat",
			adapter: NetworkAdapter{Type: NetworkTypeNAT},
		},
		{
			name:    "bridged with host interface",
			adapter: NetworkAdapter{Type: NetworkTypeBridged, HostInterface: "enp0s3"},
		},
		{
			name:    "bridged without host interface",
			adapter: NetworkAdapter{Type: NetworkTypeBridged},
			wantErr: true,
		},
		{
			name:    "unsupported type",
			adapter: NetworkAdapter{Type: "hostonly"},
			wantErr: true,
		},
		{
			name:    "invalid promiscuous mode",
			adapter: NetworkAdapter{Type: NetworkTypeNAT, PromiscuousMode: "allow-everything"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateNetworkAdapter(tt.adapter)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateNetworkAdapter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNetworkAdaptersEqual(t *testing.T) {
	t.Parallel()

	a := []NetworkAdapter{
		{Type: NetworkTypeNAT, PromiscuousMode: PromiscuousModeDeny},
		{Type: NetworkTypeBridged, HostInterface: "enp0s3", PromiscuousMode: PromiscuousModeAllowVMs},
	}
	b := []NetworkAdapter{
		{Type: NetworkTypeNAT, PromiscuousMode: PromiscuousModeDeny},
		{Type: NetworkTypeBridged, HostInterface: "enp0s3", PromiscuousMode: PromiscuousModeAllowVMs},
	}
	c := []NetworkAdapter{
		{Type: NetworkTypeBridged, HostInterface: "eth0"},
	}

	if !NetworkAdaptersEqual(a, b) {
		t.Fatal("expected adapter lists to be equal")
	}
	if NetworkAdaptersEqual(a, c) {
		t.Fatal("expected adapter lists to differ")
	}
}
