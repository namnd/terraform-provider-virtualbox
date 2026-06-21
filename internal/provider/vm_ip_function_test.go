// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func TestParseVMIPOptionsDefaults(t *testing.T) {
	t.Parallel()

	adapterIndex, timeoutValue, parsedTimeout, err := parseVMIPOptions(types.Int64Null(), types.StringNull())
	if err != nil {
		t.Fatalf("parseVMIPOptions() error = %v", err)
	}
	if adapterIndex != 0 {
		t.Fatalf("adapterIndex = %d, want 0", adapterIndex)
	}
	if timeoutValue != "60s" {
		t.Fatalf("timeoutValue = %q, want 60s", timeoutValue)
	}
	if parsedTimeout != vboxmanage.DefaultVMIPLookupTimeout() {
		t.Fatalf("parsedTimeout = %s, want %s", parsedTimeout, vboxmanage.DefaultVMIPLookupTimeout())
	}
}

func TestParseVMIPOptionsCustomValues(t *testing.T) {
	t.Parallel()

	adapterIndex, timeoutValue, parsedTimeout, err := parseVMIPOptions(
		types.Int64Value(1),
		types.StringValue("2m"),
	)
	if err != nil {
		t.Fatalf("parseVMIPOptions() error = %v", err)
	}
	if adapterIndex != 1 {
		t.Fatalf("adapterIndex = %d, want 1", adapterIndex)
	}
	if timeoutValue != "2m" {
		t.Fatalf("timeoutValue = %q, want 2m", timeoutValue)
	}
	if parsedTimeout != 2*time.Minute {
		t.Fatalf("parsedTimeout = %s, want %s", parsedTimeout, 2*time.Minute)
	}
}

func TestParseVMIPOptionsRejectsInvalidTimeout(t *testing.T) {
	t.Parallel()

	if _, _, _, err := parseVMIPOptions(types.Int64Null(), types.StringValue("not-a-duration")); err == nil {
		t.Fatal("expected invalid timeout to return an error")
	}
}

func TestVMIPFunctionRunUsesVirtualBox(t *testing.T) {
	t.Parallel()

	var readID string
	var adapterIndex int
	var timeout time.Duration

	mock := &mockVirtualBox{
		getVMIPFn: func(_ context.Context, id string, opts vboxmanage.GetVMIPOptions) (*vboxmanage.VMIP, error) {
			readID = id
			adapterIndex = opts.NetworkAdapter
			timeout = opts.Timeout
			return &vboxmanage.VMIP{
				IPAddress:  "192.168.56.101",
				MACAddress: "08:00:27:EE:A5:E7",
			}, nil
		},
	}

	fn := NewVMIPFunction(&VirtualboxProvider{vbox: mock})
	runResp := &function.RunResponse{
		Result: function.NewResultData(basetypes.NewObjectUnknown(vmIPResultAttributeTypes)),
	}

	fn.Run(context.Background(), function.RunRequest{
		Arguments: function.NewArgumentsData([]attr.Value{
			types.StringValue("00000000-0000-0000-0000-000000000001"),
			types.Int64Null(),
			types.StringValue("2m"),
		}),
	}, runResp)

	if runResp.Error != nil {
		t.Fatalf("Run() error = %v", runResp.Error)
	}
	if readID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("readID = %q, want vm uuid", readID)
	}
	if adapterIndex != 0 {
		t.Fatalf("adapterIndex = %d, want 0", adapterIndex)
	}
	if timeout != 2*time.Minute {
		t.Fatalf("timeout = %s, want %s", timeout, 2*time.Minute)
	}

	result, ok := runResp.Result.Value().(types.Object)
	if !ok {
		t.Fatalf("result type = %T, want types.Object", runResp.Result.Value())
	}
	got, diags := result.ToObjectValue(context.Background())
	if diags.HasError() {
		t.Fatalf("ToObjectValue() diagnostics = %v", diags)
	}
	if got.Attributes()["ip_address"] != types.StringValue("192.168.56.101") {
		t.Fatalf("ip_address = %v", got.Attributes()["ip_address"])
	}
}

func TestVBoxClientUsesConfiguredClient(t *testing.T) {
	t.Parallel()

	mock := &mockVirtualBox{}
	provider := &VirtualboxProvider{vbox: mock}

	client, err := provider.vboxClient(context.Background())
	if err != nil {
		t.Fatalf("vboxClient() error = %v", err)
	}
	if client != mock {
		t.Fatal("expected configured VirtualBox client to be reused")
	}
}

func TestVMIPFunctionRunPropagatesGetVMIPError(t *testing.T) {
	t.Parallel()

	want := errors.New("boom")
	mock := &mockVirtualBox{
		getVMIPFn: func(context.Context, string, vboxmanage.GetVMIPOptions) (*vboxmanage.VMIP, error) {
			return nil, want
		},
	}

	fn := NewVMIPFunction(&VirtualboxProvider{vbox: mock})
	runResp := &function.RunResponse{
		Result: function.NewResultData(basetypes.NewObjectUnknown(vmIPResultAttributeTypes)),
	}

	fn.Run(context.Background(), function.RunRequest{
		Arguments: function.NewArgumentsData([]attr.Value{
			types.StringValue("test-vm"),
			types.Int64Null(),
			types.StringNull(),
		}),
	}, runResp)

	if runResp.Error == nil {
		t.Fatal("expected Run() to return an error")
	}
}
