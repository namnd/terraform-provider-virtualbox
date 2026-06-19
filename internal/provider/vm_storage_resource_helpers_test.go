// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func TestVMStorageModelAfterReadPreservesConfiguredValues(t *testing.T) {
	t.Parallel()

	previous := vmStorageResourceModel{
		ID:          types.StringValue("vm-uuid/IDE Controller/1/0"),
		VMID:        types.StringValue("vm-uuid"),
		Name:        types.StringValue("IDE Controller"),
		Type:        types.StringValue("ide"),
		Controller:  types.StringValue("PIIX4"),
		PortCount:   types.Int64Value(0),
		HostIOCache: types.BoolValue(true),
		Bootable:    types.BoolValue(false),
		StorageAttachment: storageAttachmentModel{
			Port:   types.Int64Value(1),
			Device: types.Int64Value(0),
			Type:   types.StringValue("dvddrive"),
			Medium: types.StringValue("./metal-amd64.iso"),
		},
		LastUpdated: types.StringValue("now"),
	}

	storage := &vboxmanage.StorageCtl{
		Name:        "IDE Controller",
		Type:        vboxmanage.StorageTypeIDE,
		Controller:  "PIIX4",
		PortCount:   2,
		HostIOCache: false,
		Bootable:    true,
		Attachment: vboxmanage.StorageAttach{
			Port:   1,
			Device: 0,
			Type:   "dvddrive",
			Medium: "/absolute/path/metal-amd64.iso",
		},
	}

	model := vmStorageModelAfterRead(previous, "vm-uuid", storage)

	if model.PortCount.ValueInt64() != 0 {
		t.Fatalf("PortCount = %d, want 0", model.PortCount.ValueInt64())
	}
	if !model.HostIOCache.ValueBool() {
		t.Fatal("expected HostIOCache to remain true")
	}
	if model.Bootable.ValueBool() {
		t.Fatal("expected Bootable to remain false")
	}
	if model.StorageAttachment.Medium.ValueString() != "./metal-amd64.iso" {
		t.Fatalf("Medium = %q, want configured path", model.StorageAttachment.Medium.ValueString())
	}
}

func TestVMStorageModelAfterReadPreservesTypeAndController(t *testing.T) {
	t.Parallel()

	previous := vmStorageResourceModel{
		ID:         types.StringValue("vm-uuid/SATA Controller/0/0"),
		Type:       types.StringValue("sata"),
		Controller: types.StringValue("IntelAHCI"),
		StorageAttachment: storageAttachmentModel{
			Port:   types.Int64Value(0),
			Device: types.Int64Value(0),
			Type:   types.StringValue("hdd"),
			Medium: types.StringValue("/data/test.vdi"),
		},
	}

	storage := &vboxmanage.StorageCtl{
		Name:       "SATA Controller",
		Type:       vboxmanage.StorageTypeIDE,
		Controller: "IntelAhci",
		Attachment: vboxmanage.StorageAttach{
			Port:   0,
			Device: 0,
			Type:   "hdd",
			Medium: "/data/test.vdi",
		},
	}

	model := vmStorageModelAfterRead(previous, "vm-uuid", storage)
	if model.Type.ValueString() != "sata" {
		t.Fatalf("Type = %q, want sata", model.Type.ValueString())
	}
	if model.Controller.ValueString() != "IntelAHCI" {
		t.Fatalf("Controller = %q, want IntelAHCI", model.Controller.ValueString())
	}
}
