// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func newTestVMStorageAttachmentResource(t *testing.T, client vboxmanage.VirtualBox) *vmStorageAttachmentResource {
	t.Helper()

	return &vmStorageAttachmentResource{client: client}
}

func TestVMStorageAttachmentResourceMetadata(t *testing.T) {
	t.Parallel()

	r := NewVMStorageAttachmentResource()
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "virtualbox"}, resp)

	if resp.TypeName != "virtualbox_vm_storage_attachment" {
		t.Fatalf("TypeName = %q, want %q", resp.TypeName, "virtualbox_vm_storage_attachment")
	}
}

func TestVMStorageAttachmentResourceCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := vmStorageAttachmentTestSchema(t)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			createStorageAttachmentFunc: func(_ context.Context, vmID string, opts vboxmanage.CreateStorageAttachmentOptions) (*vboxmanage.StorageAttachment, error) {
				if vmID != "uuid-vm-1" {
					t.Fatalf("vmID = %q, want %q", vmID, "uuid-vm-1")
				}
				if opts.ControllerName != "SATA Controller" {
					t.Fatalf("ControllerName = %q, want %q", opts.ControllerName, "SATA Controller")
				}
				if opts.Port != 0 || opts.Device != 0 {
					t.Fatalf("Port/Device = %d/%d, want 0/0", opts.Port, opts.Device)
				}
				if opts.Medium != "disk-uuid" {
					t.Fatalf("Medium = %q, want %q", opts.Medium, "disk-uuid")
				}
				return &vboxmanage.StorageAttachment{
					VMID:           vmID,
					ControllerName: opts.ControllerName,
					Port:           opts.Port,
					Device:         opts.Device,
					Type:           vboxmanage.StorageAttachmentTypeHDD,
					Medium:         "/data/boot.vdi",
					MediumType:     vboxmanage.StorageMediumTypeNormal,
				}, nil
			},
		}
		r := newTestVMStorageAttachmentResource(t, mock)
		plan := vmStorageAttachmentTestPlan(t, schema, vmStorageAttachmentTestAttributeValues{
			Strings: map[string]types.String{
				"vm_id":           types.StringValue("uuid-vm-1"),
				"controller_name": types.StringValue("SATA Controller"),
				"medium":          types.StringValue("disk-uuid"),
			},
			Int64s: map[string]types.Int64{
				"port":   types.Int64Value(0),
				"device": types.Int64Value(0),
			},
		})

		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}
		r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}

		state := vmStorageAttachmentGetStateModel(t, ctx, resp.State)
		if state.ID.ValueString() != "uuid-vm-1/SATA Controller/0/0" {
			t.Fatalf("ID = %q, want %q", state.ID.ValueString(), "uuid-vm-1/SATA Controller/0/0")
		}
		if state.Medium.ValueString() != "/data/boot.vdi" {
			t.Fatalf("Medium = %q, want %q", state.Medium.ValueString(), "/data/boot.vdi")
		}
		if mock.createStorageAttachmentCalls != 1 {
			t.Fatalf("createStorageAttachmentCalls = %d, want 1", mock.createStorageAttachmentCalls)
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			createStorageAttachmentFunc: func(context.Context, string, vboxmanage.CreateStorageAttachmentOptions) (*vboxmanage.StorageAttachment, error) {
				return nil, errors.New("controller missing")
			},
		}
		r := newTestVMStorageAttachmentResource(t, mock)
		plan := vmStorageAttachmentTestPlan(t, schema, vmStorageAttachmentTestAttributeValues{
			Strings: map[string]types.String{
				"vm_id":           types.StringValue("uuid-vm-1"),
				"controller_name": types.StringValue("SATA Controller"),
				"medium":          types.StringValue("disk-uuid"),
			},
			Int64s: map[string]types.Int64{
				"port":   types.Int64Value(0),
				"device": types.Int64Value(0),
			},
		})

		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}
		r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error")
		}
	})
}

func TestVMStorageAttachmentResourceRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := vmStorageAttachmentTestSchema(t)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{}
		r := newTestVMStorageAttachmentResource(t, mock)
		state := vmStorageAttachmentTestState(t, schema, vmStorageAttachmentTestAttributeValues{
			Strings: map[string]types.String{
				"id":              types.StringValue("uuid-vm-1/SATA Controller/0/0"),
				"vm_id":           types.StringValue("uuid-vm-1"),
				"controller_name": types.StringValue("SATA Controller"),
			},
			Int64s: map[string]types.Int64{
				"port":   types.Int64Value(0),
				"device": types.Int64Value(0),
			},
		})

		resp := &resource.ReadResponse{State: state}
		r.Read(ctx, resource.ReadRequest{State: state}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if mock.getStorageAttachmentCalls != 1 {
			t.Fatalf("getStorageAttachmentCalls = %d, want 1", mock.getStorageAttachmentCalls)
		}
	})

	t.Run("attachment not found removes resource", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			getStorageAttachmentFunc: func(context.Context, string, string, int, int) (*vboxmanage.StorageAttachment, error) {
				return nil, vboxmanage.ErrStorageAttachmentNotFound
			},
		}
		r := newTestVMStorageAttachmentResource(t, mock)
		state := vmStorageAttachmentTestState(t, schema, vmStorageAttachmentTestAttributeValues{
			Strings: map[string]types.String{
				"vm_id":           types.StringValue("uuid-vm-1"),
				"controller_name": types.StringValue("SATA Controller"),
			},
			Int64s: map[string]types.Int64{
				"port":   types.Int64Value(0),
				"device": types.Int64Value(0),
			},
		})

		resp := &resource.ReadResponse{State: state}
		r.Read(ctx, resource.ReadRequest{State: state}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if !resp.State.Raw.IsNull() {
			t.Fatal("expected state to be removed")
		}
	})
}

func TestVMStorageAttachmentResourceDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := vmStorageAttachmentTestSchema(t)
	mock := &mockVirtualBox{}
	r := newTestVMStorageAttachmentResource(t, mock)
	state := vmStorageAttachmentTestState(t, schema, vmStorageAttachmentTestAttributeValues{
		Strings: map[string]types.String{
			"vm_id":           types.StringValue("uuid-vm-1"),
			"controller_name": types.StringValue("SATA Controller"),
		},
		Int64s: map[string]types.Int64{
			"port":   types.Int64Value(0),
			"device": types.Int64Value(0),
		},
	})

	resp := &resource.DeleteResponse{}
	r.Delete(ctx, resource.DeleteRequest{State: state}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if mock.deleteStorageAttachmentCalls != 1 {
		t.Fatalf("deleteStorageAttachmentCalls = %d, want 1", mock.deleteStorageAttachmentCalls)
	}
}
