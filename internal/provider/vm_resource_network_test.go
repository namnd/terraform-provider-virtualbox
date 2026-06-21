// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func TestMacAddressesFromAdapters(t *testing.T) {
	t.Parallel()

	macAddresses, diags := macAddressesFromAdapters([]vboxmanage.NetworkAdapter{
		{Type: "nat", PromiscuousMode: "deny", MACAddress: "08:00:27:EE:A5:E7"},
		{Type: "bridged", HostInterface: "enp0s3", PromiscuousMode: "allow-vms", MACAddress: "08:00:27:41:A4:F8"},
	})
	if diags.HasError() {
		t.Fatalf("macAddressesFromAdapters() diagnostics = %v", diags.Errors())
	}
	elems := macAddresses.Elements()
	if len(elems) != 2 {
		t.Fatalf("len(macAddresses) = %d, want %d", len(elems), 2)
	}

	mac0, ok := elems[0].(types.String)
	if !ok {
		t.Fatalf("macAddresses[0] is %T, want types.String", elems[0])
	}
	if mac0.ValueString() != "08:00:27:EE:A5:E7" {
		t.Fatalf("macAddresses[0] = %q, want %q", mac0.ValueString(), "08:00:27:EE:A5:E7")
	}

	mac1, ok := elems[1].(types.String)
	if !ok {
		t.Fatalf("macAddresses[1] is %T, want types.String", elems[1])
	}
	if mac1.ValueString() != "08:00:27:41:A4:F8" {
		t.Fatalf("macAddresses[1] = %q, want %q", mac1.ValueString(), "08:00:27:41:A4:F8")
	}
}

func TestNetworkAdaptersFromModel(t *testing.T) {
	t.Parallel()

	adapters, diags := networkAdaptersFromModel([]networkAdapterModel{
		{Type: types.StringValue("nat"), PromiscuousMode: types.StringValue("deny")},
		{
			Type:            types.StringValue("bridged"),
			HostInterface:   types.StringValue("enp0s3"),
			PromiscuousMode: types.StringValue("allow-vms"),
		},
	})
	if diags.HasError() {
		t.Fatalf("networkAdaptersFromModel() diagnostics = %v", diags.Errors())
	}
	if len(adapters) != 2 {
		t.Fatalf("len(adapters) = %d, want %d", len(adapters), 2)
	}
	if adapters[1].HostInterface != "enp0s3" {
		t.Fatalf("adapters[1].HostInterface = %q, want %q", adapters[1].HostInterface, "enp0s3")
	}
	if adapters[1].PromiscuousMode != "allow-vms" {
		t.Fatalf("adapters[1].PromiscuousMode = %q, want %q", adapters[1].PromiscuousMode, "allow-vms")
	}
}

func TestNetworkAdaptersFromModelRejectsInvalidPromiscuousMode(t *testing.T) {
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
}

func TestNetworkAdaptersFromModelRejectsBridgedWithoutHostInterface(t *testing.T) {
	t.Parallel()

	_, diags := networkAdaptersFromModel([]networkAdapterModel{
		{Type: types.StringValue("bridged")},
	})
	if !diags.HasError() {
		t.Fatal("expected diagnostics error for bridged adapter without host_interface")
	}
}

func TestNetworkAdaptersModelEqual(t *testing.T) {
	t.Parallel()

	plan := []networkAdapterModel{
		{
			Type:            types.StringValue("nat"),
			HostInterface:   types.StringNull(),
			PromiscuousMode: types.StringValue("deny"),
		},
	}
	state := []networkAdapterModel{
		{
			Type:            types.StringValue("nat"),
			HostInterface:   types.StringNull(),
			PromiscuousMode: types.StringValue("deny"),
		},
	}
	other := []networkAdapterModel{
		{Type: types.StringValue("bridged"), HostInterface: types.StringValue("enp0s3")},
	}

	if !networkAdaptersModelEqual(plan, state) {
		t.Fatal("expected equal adapter models")
	}
	if networkAdaptersModelEqual(plan, other) {
		t.Fatal("expected different adapter models")
	}
}
