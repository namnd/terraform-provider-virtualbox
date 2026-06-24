// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func TestNetworkAdaptersFromModel(t *testing.T) {
	t.Parallel()

	t.Run("valid nat adapter", func(t *testing.T) {
		t.Parallel()

		adapters, diags := networkAdaptersFromModel([]networkAdapterModel{
			{
				Type:            types.StringValue("nat"),
				PromiscuousMode: types.StringValue("deny"),
			},
		})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if len(adapters) != 1 {
			t.Fatalf("len(adapters) = %d, want 1", len(adapters))
		}
		if adapters[0].Type != "nat" {
			t.Fatalf("adapters[0].Type = %q, want %q", adapters[0].Type, "nat")
		}
		if adapters[0].PromiscuousMode != "deny" {
			t.Fatalf("adapters[0].PromiscuousMode = %q, want %q", adapters[0].PromiscuousMode, "deny")
		}
	})

	t.Run("valid bridged adapter", func(t *testing.T) {
		t.Parallel()

		adapters, diags := networkAdaptersFromModel([]networkAdapterModel{
			{
				Type:            types.StringValue("bridged"),
				HostInterface:   types.StringValue("eth0"),
				PromiscuousMode: types.StringValue("allow-all"),
			},
		})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if adapters[0].Type != "bridged" {
			t.Fatalf("adapters[0].Type = %q, want %q", adapters[0].Type, "bridged")
		}
		if adapters[0].HostInterface != "eth0" {
			t.Fatalf("adapters[0].HostInterface = %q, want %q", adapters[0].HostInterface, "eth0")
		}
		if adapters[0].PromiscuousMode != "allow-all" {
			t.Fatalf("adapters[0].PromiscuousMode = %q, want %q", adapters[0].PromiscuousMode, "allow-all")
		}
	})

	t.Run("bridged without host_interface", func(t *testing.T) {
		t.Parallel()

		_, diags := networkAdaptersFromModel([]networkAdapterModel{
			{Type: types.StringValue("bridged")},
		})
		if !diags.HasError() {
			t.Fatal("expected diagnostics error for bridged adapter without host_interface")
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		t.Parallel()

		_, diags := networkAdaptersFromModel([]networkAdapterModel{
			{Type: types.StringValue("hostonly")},
		})
		if !diags.HasError() {
			t.Fatal("expected diagnostics error for invalid adapter type")
		}
	})

	t.Run("invalid promiscuous_mode", func(t *testing.T) {
		t.Parallel()

		_, diags := networkAdaptersFromModel([]networkAdapterModel{
			{
				Type:            types.StringValue("nat"),
				PromiscuousMode: types.StringValue("invalid"),
			},
		})
		if !diags.HasError() {
			t.Fatal("expected diagnostics error for invalid promiscuous_mode")
		}
	})

	t.Run("reports index for invalid adapter", func(t *testing.T) {
		t.Parallel()

		_, diags := networkAdaptersFromModel([]networkAdapterModel{
			{Type: types.StringValue("nat")},
			{Type: types.StringValue("bridged")},
		})
		if !diags.HasError() {
			t.Fatal("expected diagnostics error for second adapter")
		}
		if diags.Errors()[0].Summary() != "Invalid network adapter" {
			t.Fatalf("error summary = %q, want %q", diags.Errors()[0].Summary(), "Invalid network adapter")
		}
		if diags.Errors()[0].Detail() != "network_adapter[1]: host_interface is required for bridged network adapters" {
			t.Fatalf("error detail = %q", diags.Errors()[0].Detail())
		}
	})
}

func TestNetworkAdaptersToModel(t *testing.T) {
	t.Parallel()

	t.Run("nat adapter without host interface", func(t *testing.T) {
		t.Parallel()

		models := networkAdaptersToModel([]vboxmanage.NetworkAdapter{
			{Type: "nat"},
		})
		if len(models) != 1 {
			t.Fatalf("len(models) = %d, want 1", len(models))
		}
		if models[0].Type.ValueString() != "nat" {
			t.Fatalf("models[0].Type = %q, want %q", models[0].Type.ValueString(), "nat")
		}
		if !models[0].HostInterface.IsNull() {
			t.Fatal("expected host_interface to be null for nat adapter")
		}
		if models[0].PromiscuousMode.ValueString() != "deny" {
			t.Fatalf("models[0].PromiscuousMode = %q, want %q", models[0].PromiscuousMode.ValueString(), "deny")
		}
		if !models[0].MACAddress.IsNull() {
			t.Fatal("expected mac_address to be null when adapter has no MAC")
		}
	})

	t.Run("adapter with mac address", func(t *testing.T) {
		t.Parallel()

		models := networkAdaptersToModel([]vboxmanage.NetworkAdapter{
			{
				Type:       "nat",
				MACAddress: "08:00:27:EE:A5:E7",
			},
		})
		if models[0].MACAddress.ValueString() != "08:00:27:EE:A5:E7" {
			t.Fatalf("models[0].MACAddress = %q, want %q", models[0].MACAddress.ValueString(), "08:00:27:EE:A5:E7")
		}
	})

	t.Run("bridged adapter with host interface", func(t *testing.T) {
		t.Parallel()

		models := networkAdaptersToModel([]vboxmanage.NetworkAdapter{
			{
				Type:            "bridged",
				HostInterface:   "wlp3s0",
				PromiscuousMode: "allow-vms",
			},
		})
		if models[0].HostInterface.ValueString() != "wlp3s0" {
			t.Fatalf("models[0].HostInterface = %q, want %q", models[0].HostInterface.ValueString(), "wlp3s0")
		}
		if models[0].PromiscuousMode.ValueString() != "allow-vms" {
			t.Fatalf("models[0].PromiscuousMode = %q, want %q", models[0].PromiscuousMode.ValueString(), "allow-vms")
		}
	})
}

func TestNetworkAdaptersModelEqual(t *testing.T) {
	t.Parallel()

	base := []networkAdapterModel{
		{
			Type:            types.StringValue("nat"),
			HostInterface:   types.StringNull(),
			PromiscuousMode: types.StringValue("deny"),
		},
		{
			Type:            types.StringValue("bridged"),
			HostInterface:   types.StringValue("eth0"),
			PromiscuousMode: types.StringValue("allow-all"),
		},
	}

	t.Run("equal adapters", func(t *testing.T) {
		t.Parallel()

		other := []networkAdapterModel{
			{
				Type:            types.StringValue("nat"),
				HostInterface:   types.StringNull(),
				PromiscuousMode: types.StringValue("deny"),
			},
			{
				Type:            types.StringValue("bridged"),
				HostInterface:   types.StringValue("eth0"),
				PromiscuousMode: types.StringValue("allow-all"),
			},
		}
		if !networkAdaptersModelEqual(base, other) {
			t.Fatal("expected adapters to be equal")
		}
	})

	t.Run("different lengths", func(t *testing.T) {
		t.Parallel()

		if networkAdaptersModelEqual(base, base[:1]) {
			t.Fatal("expected adapters with different lengths to be unequal")
		}
	})

	t.Run("different type", func(t *testing.T) {
		t.Parallel()

		other := []networkAdapterModel{
			{
				Type:            types.StringValue("bridged"),
				HostInterface:   types.StringNull(),
				PromiscuousMode: types.StringValue("deny"),
			},
		}
		if networkAdaptersModelEqual(base[:1], other) {
			t.Fatal("expected adapters with different types to be unequal")
		}
	})

	t.Run("different host interface", func(t *testing.T) {
		t.Parallel()

		other := []networkAdapterModel{
			{
				Type:            types.StringValue("bridged"),
				HostInterface:   types.StringValue("wlan0"),
				PromiscuousMode: types.StringValue("allow-all"),
			},
		}
		if networkAdaptersModelEqual(base[1:], other) {
			t.Fatal("expected adapters with different host interfaces to be unequal")
		}
	})

	t.Run("different promiscuous mode", func(t *testing.T) {
		t.Parallel()

		other := []networkAdapterModel{
			{
				Type:            types.StringValue("nat"),
				HostInterface:   types.StringNull(),
				PromiscuousMode: types.StringValue("allow-vms"),
			},
		}
		if networkAdaptersModelEqual(base[:1], other) {
			t.Fatal("expected adapters with different promiscuous modes to be unequal")
		}
	})

	t.Run("ignores mac address differences", func(t *testing.T) {
		t.Parallel()

		other := []networkAdapterModel{
			{
				Type:            types.StringValue("nat"),
				HostInterface:   types.StringNull(),
				PromiscuousMode: types.StringValue("deny"),
				MACAddress:      types.StringValue("08:00:27:EE:A5:E7"),
			},
		}
		if !networkAdaptersModelEqual(base[:1], other) {
			t.Fatal("expected mac_address differences to be ignored when comparing adapters")
		}
	})
}
