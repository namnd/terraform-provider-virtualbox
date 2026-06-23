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

func newTestDiskResource(t *testing.T, client vboxmanage.VirtualBox) *diskResource {
	t.Helper()

	return &diskResource{client: client}
}

func TestDiskResourceMetadata(t *testing.T) {
	t.Parallel()

	r := NewDiskResource()
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "virtualbox"}, resp)

	if resp.TypeName != "virtualbox_disk" {
		t.Fatalf("TypeName = %q, want %q", resp.TypeName, "virtualbox_disk")
	}
}

func TestDiskResourceConfigure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("nil provider data", func(t *testing.T) {
		t.Parallel()

		r := &diskResource{}
		resp := &resource.ConfigureResponse{}
		r.Configure(ctx, resource.ConfigureRequest{}, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
	})

	t.Run("invalid provider data type", func(t *testing.T) {
		t.Parallel()

		r := &diskResource{}
		resp := &resource.ConfigureResponse{}
		r.Configure(ctx, resource.ConfigureRequest{ProviderData: "invalid"}, resp)

		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error for invalid provider data type")
		}
	})

	t.Run("valid provider data", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{}
		r := &diskResource{}
		resp := &resource.ConfigureResponse{}
		r.Configure(ctx, resource.ConfigureRequest{ProviderData: mock}, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if r.client == nil {
			t.Fatal("expected client to be configured")
		}
	})
}

func TestDiskResourceCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := diskTestSchema(t)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			createDiskFunc: func(_ context.Context, opts vboxmanage.CreateDiskOptions) (*vboxmanage.Disk, error) {
				if opts.FilePath != "/tmp/test.vdi" {
					t.Fatalf("CreateDisk FilePath = %q, want %q", opts.FilePath, "/tmp/test.vdi")
				}
				if opts.Size != 2048 {
					t.Fatalf("CreateDisk Size = %d, want 2048", opts.Size)
				}
				if opts.Format != vboxmanage.DiskFormatVDI {
					t.Fatalf("CreateDisk Format = %q, want %q", opts.Format, vboxmanage.DiskFormatVDI)
				}
				if opts.Variant != vboxmanage.DiskVariantStandard {
					t.Fatalf("CreateDisk Variant = %q, want %q", opts.Variant, vboxmanage.DiskVariantStandard)
				}
				return &vboxmanage.Disk{
					UUID:     "uuid-123",
					FilePath: opts.FilePath,
					Size:     opts.Size,
					Format:   opts.Format,
					Variant:  opts.Variant,
				}, nil
			},
		}
		r := newTestDiskResource(t, mock)

		req := resource.CreateRequest{
			Plan: diskTestPlan(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"file_path": types.StringValue("/tmp/test.vdi"),
					"format":    types.StringValue(vboxmanage.DiskFormatVDI),
					"variant":   types.StringValue(vboxmanage.DiskVariantStandard),
				},
				Int64s: map[string]types.Int64{
					"size": types.Int64Value(2048),
				},
			}),
		}
		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}

		r.Create(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Create diagnostics: %v", resp.Diagnostics)
		}

		state := diskGetStateModel(t, ctx, resp.State)
		if state.ID.ValueString() != "uuid-123" {
			t.Fatalf("state.ID = %q, want %q", state.ID.ValueString(), "uuid-123")
		}
		if state.FilePath.ValueString() != "/tmp/test.vdi" {
			t.Fatalf("state.FilePath = %q, want %q", state.FilePath.ValueString(), "/tmp/test.vdi")
		}
		if state.Size.ValueInt64() != 2048 {
			t.Fatalf("state.Size = %d, want 2048", state.Size.ValueInt64())
		}
		if state.Format.ValueString() != vboxmanage.DiskFormatVDI {
			t.Fatalf("state.Format = %q, want %q", state.Format.ValueString(), vboxmanage.DiskFormatVDI)
		}
		if state.Variant.ValueString() != vboxmanage.DiskVariantStandard {
			t.Fatalf("state.Variant = %q, want %q", state.Variant.ValueString(), vboxmanage.DiskVariantStandard)
		}
		if state.LastUpdated.IsNull() || state.LastUpdated.IsUnknown() {
			t.Fatal("expected last_updated to be set")
		}
		if mock.createDiskCalls != 1 {
			t.Fatalf("CreateDisk calls = %d, want 1", mock.createDiskCalls)
		}
	})

	t.Run("success with custom format and variant", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			createDiskFunc: func(_ context.Context, opts vboxmanage.CreateDiskOptions) (*vboxmanage.Disk, error) {
				if opts.Format != vboxmanage.DiskFormatVMDK {
					t.Fatalf("CreateDisk Format = %q, want %q", opts.Format, vboxmanage.DiskFormatVMDK)
				}
				if opts.Variant != vboxmanage.DiskVariantFixed {
					t.Fatalf("CreateDisk Variant = %q, want %q", opts.Variant, vboxmanage.DiskVariantFixed)
				}
				return &vboxmanage.Disk{
					UUID:     "uuid-456",
					FilePath: opts.FilePath,
					Size:     opts.Size,
					Format:   opts.Format,
					Variant:  opts.Variant,
				}, nil
			},
		}
		r := newTestDiskResource(t, mock)

		req := resource.CreateRequest{
			Plan: diskTestPlan(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"file_path": types.StringValue("/tmp/fixed.vmdk"),
					"format":    types.StringValue(vboxmanage.DiskFormatVMDK),
					"variant":   types.StringValue(vboxmanage.DiskVariantFixed),
				},
				Int64s: map[string]types.Int64{
					"size": types.Int64Value(4096),
				},
			}),
		}
		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}

		r.Create(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Create diagnostics: %v", resp.Diagnostics)
		}

		state := diskGetStateModel(t, ctx, resp.State)
		if state.Format.ValueString() != vboxmanage.DiskFormatVMDK {
			t.Fatalf("state.Format = %q, want %q", state.Format.ValueString(), vboxmanage.DiskFormatVMDK)
		}
		if state.Variant.ValueString() != vboxmanage.DiskVariantFixed {
			t.Fatalf("state.Variant = %q, want %q", state.Variant.ValueString(), vboxmanage.DiskVariantFixed)
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			createDiskFunc: func(context.Context, vboxmanage.CreateDiskOptions) (*vboxmanage.Disk, error) {
				return nil, errors.New("create failed")
			},
		}
		r := newTestDiskResource(t, mock)

		req := resource.CreateRequest{
			Plan: diskTestPlan(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
				Int64s: map[string]types.Int64{
					"size": types.Int64Value(1024),
				},
			}),
		}
		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}

		r.Create(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when create fails")
		}
	})
}

func TestDiskResourceRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := diskTestSchema(t)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			getDiskFunc: func(_ context.Context, id string) (*vboxmanage.Disk, error) {
				if id != "/tmp/test.vdi" {
					t.Fatalf("GetDisk id = %q, want %q", id, "/tmp/test.vdi")
				}
				return &vboxmanage.Disk{
					UUID:     "uuid-123",
					FilePath: "/tmp/test.vdi",
					Size:     4096,
					Format:   vboxmanage.DiskFormatVHD,
					Variant:  vboxmanage.DiskVariantFixed,
				}, nil
			},
		}
		r := newTestDiskResource(t, mock)

		req := resource.ReadRequest{
			State: diskTestState(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"id":        types.StringValue("uuid-123"),
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
				Int64s: map[string]types.Int64{
					"size": types.Int64Value(2048),
				},
			}),
		}
		resp := &resource.ReadResponse{State: tfsdk.State{Schema: schema}}

		r.Read(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics)
		}

		state := diskGetStateModel(t, ctx, resp.State)
		if state.ID.ValueString() != "uuid-123" {
			t.Fatalf("state.ID = %q, want %q", state.ID.ValueString(), "uuid-123")
		}
		if state.Size.ValueInt64() != 4096 {
			t.Fatalf("state.Size = %d, want 4096", state.Size.ValueInt64())
		}
		if state.Format.ValueString() != vboxmanage.DiskFormatVHD {
			t.Fatalf("state.Format = %q, want %q", state.Format.ValueString(), vboxmanage.DiskFormatVHD)
		}
		if state.Variant.ValueString() != vboxmanage.DiskVariantFixed {
			t.Fatalf("state.Variant = %q, want %q", state.Variant.ValueString(), vboxmanage.DiskVariantFixed)
		}
	})

	t.Run("disk not found removes resource", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			getDiskFunc: func(context.Context, string) (*vboxmanage.Disk, error) {
				return nil, vboxmanage.ErrMediumNotFound
			},
		}
		r := newTestDiskResource(t, mock)

		req := resource.ReadRequest{
			State: diskTestState(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"id":        types.StringValue("uuid-123"),
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
			}),
		}
		resp := &resource.ReadResponse{State: tfsdk.State{Schema: schema}}

		r.Read(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics)
		}
		if !resp.State.Raw.IsNull() {
			t.Fatal("expected state to be removed when disk is not found")
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			getDiskFunc: func(context.Context, string) (*vboxmanage.Disk, error) {
				return nil, errors.New("read failed")
			},
		}
		r := newTestDiskResource(t, mock)

		req := resource.ReadRequest{
			State: diskTestState(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"id":        types.StringValue("uuid-123"),
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
			}),
		}
		resp := &resource.ReadResponse{State: tfsdk.State{Schema: schema}}

		r.Read(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when read fails")
		}
	})
}

func TestDiskResourceUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := diskTestSchema(t)

	t.Run("resizes disk", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			updateDiskFunc: func(_ context.Context, id string, opts vboxmanage.UpdateDiskOptions) (*vboxmanage.Disk, error) {
				if id != "/tmp/test.vdi" {
					t.Fatalf("UpdateDisk id = %q, want %q", id, "/tmp/test.vdi")
				}
				if opts.Size == nil || *opts.Size != 4096 {
					t.Fatalf("UpdateDisk Size = %v, want 4096", opts.Size)
				}
				return &vboxmanage.Disk{
					UUID:     "uuid-123",
					FilePath: "/tmp/test.vdi",
					Size:     4096,
					Format:   vboxmanage.DiskFormatVDI,
					Variant:  vboxmanage.DiskVariantStandard,
				}, nil
			},
		}
		r := newTestDiskResource(t, mock)

		req := resource.UpdateRequest{
			Plan: diskTestPlan(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
				Int64s: map[string]types.Int64{
					"size": types.Int64Value(4096),
				},
			}),
			State: diskTestState(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"id":        types.StringValue("uuid-123"),
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
				Int64s: map[string]types.Int64{
					"size": types.Int64Value(2048),
				},
			}),
		}
		resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schema}}

		r.Update(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Update diagnostics: %v", resp.Diagnostics)
		}

		state := diskGetStateModel(t, ctx, resp.State)
		if state.ID.ValueString() != "uuid-123" {
			t.Fatalf("state.ID = %q, want %q", state.ID.ValueString(), "uuid-123")
		}
		if state.Size.ValueInt64() != 4096 {
			t.Fatalf("state.Size = %d, want 4096", state.Size.ValueInt64())
		}
		if state.LastUpdated.IsNull() || state.LastUpdated.IsUnknown() {
			t.Fatal("expected last_updated to be set")
		}
		if mock.updateDiskCalls != 1 {
			t.Fatalf("UpdateDisk calls = %d, want 1", mock.updateDiskCalls)
		}
	})

	t.Run("skips update when unchanged", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{}
		r := newTestDiskResource(t, mock)

		req := resource.UpdateRequest{
			Plan: diskTestPlan(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
				Int64s: map[string]types.Int64{
					"size": types.Int64Value(2048),
				},
			}),
			State: diskTestState(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"id":        types.StringValue("uuid-123"),
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
				Int64s: map[string]types.Int64{
					"size": types.Int64Value(2048),
				},
			}),
		}
		resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schema}}

		r.Update(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Update diagnostics: %v", resp.Diagnostics)
		}
		if mock.updateDiskCalls != 0 {
			t.Fatalf("UpdateDisk calls = %d, want 0", mock.updateDiskCalls)
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			updateDiskFunc: func(context.Context, string, vboxmanage.UpdateDiskOptions) (*vboxmanage.Disk, error) {
				return nil, errors.New("update failed")
			},
		}
		r := newTestDiskResource(t, mock)

		req := resource.UpdateRequest{
			Plan: diskTestPlan(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
				Int64s: map[string]types.Int64{
					"size": types.Int64Value(4096),
				},
			}),
			State: diskTestState(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"id":        types.StringValue("uuid-123"),
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
				Int64s: map[string]types.Int64{
					"size": types.Int64Value(2048),
				},
			}),
		}
		resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schema}}

		r.Update(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when update fails")
		}
	})
}

func TestDiskResourceDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := diskTestSchema(t)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			deleteDiskFunc: func(_ context.Context, id string) error {
				if id != "/tmp/test.vdi" {
					t.Fatalf("DeleteDisk id = %q, want %q", id, "/tmp/test.vdi")
				}
				return nil
			},
		}
		r := newTestDiskResource(t, mock)

		req := resource.DeleteRequest{
			State: diskTestState(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"id":        types.StringValue("uuid-123"),
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
			}),
		}
		resp := &resource.DeleteResponse{}

		r.Delete(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Delete diagnostics: %v", resp.Diagnostics)
		}
		if mock.deleteDiskCalls != 1 {
			t.Fatalf("DeleteDisk calls = %d, want 1", mock.deleteDiskCalls)
		}
	})

	t.Run("disk not found is ignored", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			deleteDiskFunc: func(context.Context, string) error {
				return vboxmanage.ErrMediumNotFound
			},
		}
		r := newTestDiskResource(t, mock)

		req := resource.DeleteRequest{
			State: diskTestState(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"id":        types.StringValue("uuid-123"),
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
			}),
		}
		resp := &resource.DeleteResponse{}

		r.Delete(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Delete diagnostics: %v", resp.Diagnostics)
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			deleteDiskFunc: func(context.Context, string) error {
				return errors.New("delete failed")
			},
		}
		r := newTestDiskResource(t, mock)

		req := resource.DeleteRequest{
			State: diskTestState(t, schema, diskTestAttributeValues{
				Strings: map[string]types.String{
					"id":        types.StringValue("uuid-123"),
					"file_path": types.StringValue("/tmp/test.vdi"),
				},
			}),
		}
		resp := &resource.DeleteResponse{}

		r.Delete(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when delete fails")
		}
	})
}
