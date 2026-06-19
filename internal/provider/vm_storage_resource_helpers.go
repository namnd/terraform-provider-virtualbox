// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func vmStorageResourceID(vmID, name string, port, device int64) string {
	return fmt.Sprintf("%s/%s/%d/%d", vmID, name, port, device)
}

func vmStorageFromModel(model vmStorageResourceModel) (vboxmanage.StorageCtl, diag.Diagnostics) {
	var diags diag.Diagnostics

	ctl := vboxmanage.StorageCtl{
		Name:        model.Name.ValueString(),
		Type:        model.Type.ValueString(),
		Controller:  model.Controller.ValueString(),
		PortCount:   int(model.PortCount.ValueInt64()),
		HostIOCache: model.HostIOCache.ValueBool(),
		Bootable:    model.Bootable.ValueBool(),
		Attachment: vboxmanage.StorageAttach{
			Port:   int(model.StorageAttachment.Port.ValueInt64()),
			Device: int(model.StorageAttachment.Device.ValueInt64()),
			Type:   model.StorageAttachment.Type.ValueString(),
			Medium: model.StorageAttachment.Medium.ValueString(),
		},
	}
	ctl.Controller = vboxmanage.NormalizeStorageController(ctl.Type, ctl.Controller)

	if err := vboxmanage.ValidateStorageCtlWithAttachment(ctl); err != nil {
		diags.AddError("Invalid VM storage", err.Error())
	}

	return ctl, diags
}

func vmStorageModelAfterRead(previous vmStorageResourceModel, vmID string, storage *vboxmanage.StorageCtl) vmStorageResourceModel {
	model := vmStorageToModel(vmID, storage)
	model.ID = previous.ID
	model.LastUpdated = previous.LastUpdated

	// VirtualBox does not expose host I/O cache in machine-readable showvminfo output.
	if !previous.HostIOCache.IsNull() {
		model.HostIOCache = previous.HostIOCache
	}
	// Preserve configured controller metadata; VirtualBox uses different chipset casing and
	// defaults that do not round-trip cleanly through showvminfo.
	if !previous.Type.IsNull() {
		model.Type = previous.Type
	}
	if !previous.Controller.IsNull() {
		model.Controller = previous.Controller
	}
	// Preserve the configured port_count when VirtualBox reports its resolved default instead.
	if !previous.PortCount.IsNull() {
		model.PortCount = previous.PortCount
	}
	// Preserve the configured bootable flag; the first IDE controller is bootable by default in VirtualBox.
	if !previous.Bootable.IsNull() {
		model.Bootable = previous.Bootable
	}
	// Preserve the configured medium path; VirtualBox stores an absolute path.
	if !previous.StorageAttachment.Medium.IsNull() {
		model.StorageAttachment.Medium = previous.StorageAttachment.Medium
	}

	return model
}

func vmStorageToModel(vmID string, storage *vboxmanage.StorageCtl) vmStorageResourceModel {
	attach := storage.Attachment
	return vmStorageResourceModel{
		ID:          types.StringValue(vmStorageResourceID(vmID, storage.Name, int64(attach.Port), int64(attach.Device))),
		VMID:        types.StringValue(vmID),
		Name:        types.StringValue(storage.Name),
		Type:        types.StringValue(storage.Type),
		Controller:  types.StringValue(storage.Controller),
		PortCount:   types.Int64Value(int64(storage.PortCount)),
		HostIOCache: types.BoolValue(storage.HostIOCache),
		Bootable:    types.BoolValue(storage.Bootable),
		StorageAttachment: storageAttachmentModel{
			Port:   types.Int64Value(int64(attach.Port)),
			Device: types.Int64Value(int64(attach.Device)),
			Type:   types.StringValue(attach.Type),
			Medium: types.StringValue(attach.Medium),
		},
	}
}

func vmStorageDeleteFromState(state vmStorageResourceModel) (vboxmanage.StorageCtl, diag.Diagnostics) {
	var diags diag.Diagnostics

	if state.VMID.IsNull() || strings.TrimSpace(state.VMID.ValueString()) == "" {
		diags.AddError("Invalid VM storage state", "vm_id must not be empty")
	}
	if state.Name.IsNull() || strings.TrimSpace(state.Name.ValueString()) == "" {
		diags.AddError("Invalid VM storage state", "name must not be empty")
	}
	if state.StorageAttachment.Port.IsNull() || state.StorageAttachment.Device.IsNull() {
		diags.AddError("Invalid VM storage state", "storage_attachment port and device must not be empty")
	}
	if diags.HasError() {
		return vboxmanage.StorageCtl{}, diags
	}

	return vboxmanage.StorageCtl{
		Name: state.Name.ValueString(),
		Attachment: vboxmanage.StorageAttach{
			Port:   int(state.StorageAttachment.Port.ValueInt64()),
			Device: int(state.StorageAttachment.Device.ValueInt64()),
			Type:   state.StorageAttachment.Type.ValueString(),
		},
	}, diags
}

func vmStorageAttachmentChanged(plan, state vmStorageResourceModel) bool {
	return !plan.StorageAttachment.Type.Equal(state.StorageAttachment.Type) ||
		!plan.StorageAttachment.Medium.Equal(state.StorageAttachment.Medium)
}
