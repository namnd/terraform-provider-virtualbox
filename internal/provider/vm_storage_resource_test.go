// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func TestVMStorageResourceConfigureAcceptsVirtualBox(t *testing.T) {
	t.Parallel()

	mock := &mockVirtualBox{}
	r := &vmStorageResource{}
	resp := &resource.ConfigureResponse{}

	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: mock,
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure() diagnostics = %v", resp.Diagnostics.Errors())
	}
	if r.vbox != mock {
		t.Fatal("expected mock VirtualBox to be stored on resource")
	}
}

func TestVMStorageFromModel(t *testing.T) {
	t.Parallel()

	model := vmStorageResourceModel{
		Name:        types.StringValue("IDE Controller"),
		Type:        types.StringValue("ide"),
		Controller:  types.StringValue("PIIX4"),
		PortCount:   types.Int64Value(0),
		HostIOCache: types.BoolValue(true),
		Bootable:    types.BoolValue(true),
		StorageAttachment: storageAttachmentModel{
			Port:   types.Int64Value(1),
			Device: types.Int64Value(0),
			Type:   types.StringValue("dvddrive"),
			Medium: types.StringValue("/path/to/metal-amd64.iso"),
		},
	}

	ctl, diags := vmStorageFromModel(model)
	if diags.HasError() {
		t.Fatalf("vmStorageFromModel() diagnostics = %v", diags.Errors())
	}
	if ctl.Name != "IDE Controller" {
		t.Fatalf("ctl.Name = %q, want %q", ctl.Name, "IDE Controller")
	}
	if ctl.Attachment.Port != 1 || ctl.Attachment.Device != 0 {
		t.Fatalf("attachment slot = %d:%d, want 1:0", ctl.Attachment.Port, ctl.Attachment.Device)
	}
	if ctl.Attachment.Medium != "/path/to/metal-amd64.iso" {
		t.Fatalf("ctl.Attachment.Medium = %q, want expected path", ctl.Attachment.Medium)
	}
}

func TestVMStorageResourceCreateUsesVirtualBox(t *testing.T) {
	t.Parallel()

	var createdCtl vboxmanage.StorageCtl
	mock := &mockVirtualBox{
		createVMStorageFn: func(_ context.Context, _ string, ctl vboxmanage.StorageCtl) error {
			createdCtl = ctl
			return nil
		},
	}

	r := &vmStorageResource{vbox: mock}

	ctl := vboxmanage.StorageCtl{
		Name:        "IDE Controller",
		Type:        vboxmanage.StorageTypeIDE,
		Controller:  vboxmanage.StorageControllerPIIX4,
		HostIOCache: true,
		Bootable:    true,
		Attachment: vboxmanage.StorageAttach{
			Port:   1,
			Device: 0,
			Type:   vboxmanage.StorageAttachTypeDVDDrive,
			Medium: "/path/to/metal-amd64.iso",
		},
	}
	if err := r.vbox.CreateVMStorage(context.Background(), "vm-1", ctl); err != nil {
		t.Fatalf("CreateVMStorage() error = %v", err)
	}
	if createdCtl.Name != "IDE Controller" {
		t.Fatalf("createdCtl.Name = %q, want %q", createdCtl.Name, "IDE Controller")
	}
	if createdCtl.Attachment.Medium != "/path/to/metal-amd64.iso" {
		t.Fatalf("createdCtl.Attachment.Medium = %q, want expected path", createdCtl.Attachment.Medium)
	}
}

func TestVMStorageDeleteFromState(t *testing.T) {
	t.Parallel()

	ctl, diags := vmStorageDeleteFromState(vmStorageResourceModel{
		VMID: types.StringValue("vm-uuid"),
		Name: types.StringValue("IDE Controller"),
		StorageAttachment: storageAttachmentModel{
			Port:   types.Int64Value(1),
			Device: types.Int64Value(0),
			Type:   types.StringValue("dvddrive"),
		},
	})
	if diags.HasError() {
		t.Fatalf("vmStorageDeleteFromState() diagnostics = %v", diags.Errors())
	}
	if ctl.Name != "IDE Controller" {
		t.Fatalf("ctl.Name = %q, want %q", ctl.Name, "IDE Controller")
	}
	if ctl.Attachment.Port != 1 || ctl.Attachment.Device != 0 {
		t.Fatalf("attachment slot = %d:%d, want 1:0", ctl.Attachment.Port, ctl.Attachment.Device)
	}
}

func TestVMStorageResourceID(t *testing.T) {
	t.Parallel()

	got := vmStorageResourceID("vm-uuid", "IDE Controller", 1, 0)
	want := "vm-uuid/IDE Controller/1/0"
	if got != want {
		t.Fatalf("vmStorageResourceID() = %q, want %q", got, want)
	}
}
