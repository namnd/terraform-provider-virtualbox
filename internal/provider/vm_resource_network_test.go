// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

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
