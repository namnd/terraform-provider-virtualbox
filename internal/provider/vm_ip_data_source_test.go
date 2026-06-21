// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func TestVMIPDataSourceConfigureAcceptsVirtualBox(t *testing.T) {
	t.Parallel()

	mock := &mockVirtualBox{}
	d := &vmIPDataSource{}
	resp := &datasource.ConfigureResponse{}

	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: mock,
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure() diagnostics = %v", resp.Diagnostics.Errors())
	}
	if d.vbox != mock {
		t.Fatal("expected mock VirtualBox to be stored on data source")
	}
}

func TestVMIPDataSourceConfigureRejectsWrongType(t *testing.T) {
	t.Parallel()

	d := &vmIPDataSource{}
	resp := &datasource.ConfigureResponse{}

	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: "not-a-virtualbox-client",
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected Configure() to fail for wrong provider data type")
	}
}

func TestVMIPDataSourceReadUsesVirtualBox(t *testing.T) {
	t.Parallel()

	var readID string
	var adapterIndex int
	mock := &mockVirtualBox{
		getVMIPFn: func(_ context.Context, id string, opts vboxmanage.GetVMIPOptions) (*vboxmanage.VMIP, error) {
			readID = id
			adapterIndex = opts.NetworkAdapter
			return &vboxmanage.VMIP{
				IPAddress:  "192.168.56.101",
				MACAddress: "08:00:27:EE:A5:E7",
			}, nil
		},
	}

	d := &vmIPDataSource{vbox: mock}

	vmIP, err := d.vbox.GetVMIP(context.Background(), "00000000-0000-0000-0000-000000000001", vboxmanage.GetVMIPOptions{
		NetworkAdapter: 0,
	})
	if err != nil {
		t.Fatalf("GetVMIP() error = %v", err)
	}
	if readID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("readID = %q, want %q", readID, "00000000-0000-0000-0000-000000000001")
	}
	if adapterIndex != 0 {
		t.Fatalf("adapterIndex = %d, want %d", adapterIndex, 0)
	}
	if vmIP.IPAddress != "192.168.56.101" {
		t.Fatalf("vmIP.IPAddress = %q, want %q", vmIP.IPAddress, "192.168.56.101")
	}
	if vmIP.MACAddress != "08:00:27:EE:A5:E7" {
		t.Fatalf("vmIP.MACAddress = %q, want %q", vmIP.MACAddress, "08:00:27:EE:A5:E7")
	}
}

func TestMockVirtualBoxPropagatesGetVMIPError(t *testing.T) {
	t.Parallel()

	want := errors.New("boom")
	mock := &mockVirtualBox{
		getVMIPFn: func(context.Context, string, vboxmanage.GetVMIPOptions) (*vboxmanage.VMIP, error) {
			return nil, want
		},
	}

	if _, err := mock.GetVMIP(context.Background(), "test-vm", vboxmanage.GetVMIPOptions{}); !errors.Is(err, want) {
		t.Fatalf("GetVMIP() error = %v, want %v", err, want)
	}
}
