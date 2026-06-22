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

type mockVirtualBox struct {
	createVMCalls int
	updateVMCalls int
	deleteVMCalls int
	getVMCalls    int

	createVMFunc func(ctx context.Context, name string, opts vboxmanage.CreateVMOptions) (*vboxmanage.VM, error)
	getVMFunc    func(ctx context.Context, id string) (*vboxmanage.VM, error)
	updateVMFunc func(ctx context.Context, id string, opts vboxmanage.UpdateVMOptions) (*vboxmanage.VM, error)
	deleteVMFunc func(ctx context.Context, id string) error
}

func (m *mockVirtualBox) Version(context.Context) (string, error) {
	return "7.0.0", nil
}

func (m *mockVirtualBox) CreateVM(ctx context.Context, name string, opts vboxmanage.CreateVMOptions) (*vboxmanage.VM, error) {
	m.createVMCalls++
	if m.createVMFunc != nil {
		return m.createVMFunc(ctx, name, opts)
	}
	return &vboxmanage.VM{Name: name, UUID: "uuid-" + name}, nil
}

func (m *mockVirtualBox) GetVM(ctx context.Context, id string) (*vboxmanage.VM, error) {
	m.getVMCalls++
	if m.getVMFunc != nil {
		return m.getVMFunc(ctx, id)
	}
	return &vboxmanage.VM{Name: "vm-" + id, UUID: id}, nil
}

func (m *mockVirtualBox) UpdateVM(ctx context.Context, id string, opts vboxmanage.UpdateVMOptions) (*vboxmanage.VM, error) {
	m.updateVMCalls++
	if m.updateVMFunc != nil {
		return m.updateVMFunc(ctx, id, opts)
	}
	return &vboxmanage.VM{Name: opts.Name, UUID: id}, nil
}

func (m *mockVirtualBox) DeleteVM(ctx context.Context, id string) error {
	m.deleteVMCalls++
	if m.deleteVMFunc != nil {
		return m.deleteVMFunc(ctx, id)
	}
	return nil
}

func newTestVMResource(t *testing.T, client vboxmanage.VirtualBox) *vmResource {
	t.Helper()

	return &vmResource{client: client}
}

func TestVMResourceMetadata(t *testing.T) {
	t.Parallel()

	r := NewVMResource()
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "virtualbox"}, resp)

	if resp.TypeName != "virtualbox_vm" {
		t.Fatalf("TypeName = %q, want %q", resp.TypeName, "virtualbox_vm")
	}
}

func TestVMResourceConfigure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("nil provider data", func(t *testing.T) {
		t.Parallel()

		r := &vmResource{}
		resp := &resource.ConfigureResponse{}
		r.Configure(ctx, resource.ConfigureRequest{}, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
	})

	t.Run("invalid provider data type", func(t *testing.T) {
		t.Parallel()

		r := &vmResource{}
		resp := &resource.ConfigureResponse{}
		r.Configure(ctx, resource.ConfigureRequest{ProviderData: "invalid"}, resp)

		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error for invalid provider data type")
		}
	})

	t.Run("valid provider data", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{}
		r := &vmResource{}
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

func TestVMResourceCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := vmTestSchema(t)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			createVMFunc: func(_ context.Context, name string, opts vboxmanage.CreateVMOptions) (*vboxmanage.VM, error) {
				if name != "test-vm" {
					t.Fatalf("CreateVM name = %q, want %q", name, "test-vm")
				}
				if opts.OSType != "Linux_64" {
					t.Fatalf("CreateVM OSType = %q, want %q", opts.OSType, "Linux_64")
				}
				return &vboxmanage.VM{Name: name, UUID: "uuid-123"}, nil
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.CreateRequest{
			Plan: vmTestPlan(t, schema, map[string]types.String{
				"name":    types.StringValue("test-vm"),
				"os_type": types.StringValue("Linux_64"),
			}),
		}
		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}

		r.Create(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Create diagnostics: %v", resp.Diagnostics)
		}

		state := vmGetStateModel(t, ctx, resp.State)
		if state.ID.ValueString() != "uuid-123" {
			t.Fatalf("state.ID = %q, want %q", state.ID.ValueString(), "uuid-123")
		}
		if state.Name.ValueString() != "test-vm" {
			t.Fatalf("state.Name = %q, want %q", state.Name.ValueString(), "test-vm")
		}
		if state.LastUpdated.IsNull() || state.LastUpdated.IsUnknown() {
			t.Fatal("expected last_updated to be set")
		}
		if mock.createVMCalls != 1 {
			t.Fatalf("CreateVM calls = %d, want 1", mock.createVMCalls)
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			createVMFunc: func(context.Context, string, vboxmanage.CreateVMOptions) (*vboxmanage.VM, error) {
				return nil, errors.New("create failed")
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.CreateRequest{
			Plan: vmTestPlan(t, schema, map[string]types.String{
				"name":    types.StringValue("test-vm"),
				"os_type": types.StringValue("Linux_64"),
			}),
		}
		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}

		r.Create(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when create fails")
		}
	})
}

func TestVMResourceRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := vmTestSchema(t)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			getVMFunc: func(_ context.Context, id string) (*vboxmanage.VM, error) {
				if id != "uuid-123" {
					t.Fatalf("GetVM id = %q, want %q", id, "uuid-123")
				}
				return &vboxmanage.VM{Name: "updated-name", UUID: "uuid-123"}, nil
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.ReadRequest{
			State: vmTestState(t, schema, map[string]types.String{
				"id":   types.StringValue("uuid-123"),
				"name": types.StringValue("old-name"),
			}),
		}
		resp := &resource.ReadResponse{State: tfsdk.State{Schema: schema}}

		r.Read(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics)
		}

		state := vmGetStateModel(t, ctx, resp.State)
		if state.ID.ValueString() != "uuid-123" {
			t.Fatalf("state.ID = %q, want %q", state.ID.ValueString(), "uuid-123")
		}
		if state.Name.ValueString() != "updated-name" {
			t.Fatalf("state.Name = %q, want %q", state.Name.ValueString(), "updated-name")
		}
	})

	t.Run("vm not found removes resource", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			getVMFunc: func(context.Context, string) (*vboxmanage.VM, error) {
				return nil, vboxmanage.ErrVMNotFound
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.ReadRequest{
			State: vmTestState(t, schema, map[string]types.String{
				"id":   types.StringValue("uuid-123"),
				"name": types.StringValue("test-vm"),
			}),
		}
		resp := &resource.ReadResponse{State: tfsdk.State{Schema: schema}}

		r.Read(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics)
		}
		if !resp.State.Raw.IsNull() {
			t.Fatal("expected state to be removed when VM is not found")
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			getVMFunc: func(context.Context, string) (*vboxmanage.VM, error) {
				return nil, errors.New("read failed")
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.ReadRequest{
			State: vmTestState(t, schema, map[string]types.String{
				"id":   types.StringValue("uuid-123"),
				"name": types.StringValue("test-vm"),
			}),
		}
		resp := &resource.ReadResponse{State: tfsdk.State{Schema: schema}}

		r.Read(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when read fails")
		}
	})
}

func TestVMResourceUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := vmTestSchema(t)

	t.Run("renames vm", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			updateVMFunc: func(_ context.Context, id string, opts vboxmanage.UpdateVMOptions) (*vboxmanage.VM, error) {
				if id != "uuid-123" {
					t.Fatalf("UpdateVM id = %q, want %q", id, "uuid-123")
				}
				if opts.Name != "new-name" {
					t.Fatalf("UpdateVM name = %q, want %q", opts.Name, "new-name")
				}
				return &vboxmanage.VM{Name: "new-name", UUID: "uuid-123"}, nil
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.UpdateRequest{
			Plan: vmTestPlan(t, schema, map[string]types.String{
				"name": types.StringValue("new-name"),
			}),
			State: vmTestState(t, schema, map[string]types.String{
				"id":   types.StringValue("uuid-123"),
				"name": types.StringValue("old-name"),
			}),
		}
		resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schema}}

		r.Update(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Update diagnostics: %v", resp.Diagnostics)
		}

		state := vmGetStateModel(t, ctx, resp.State)
		if state.ID.ValueString() != "uuid-123" {
			t.Fatalf("state.ID = %q, want %q", state.ID.ValueString(), "uuid-123")
		}
		if state.Name.ValueString() != "new-name" {
			t.Fatalf("state.Name = %q, want %q", state.Name.ValueString(), "new-name")
		}
		if state.LastUpdated.IsNull() || state.LastUpdated.IsUnknown() {
			t.Fatal("expected last_updated to be set")
		}
		if mock.updateVMCalls != 1 {
			t.Fatalf("UpdateVM calls = %d, want 1", mock.updateVMCalls)
		}
	})

	t.Run("skips update when name unchanged", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{}
		r := newTestVMResource(t, mock)

		req := resource.UpdateRequest{
			Plan: vmTestPlan(t, schema, map[string]types.String{
				"name": types.StringValue("same-name"),
			}),
			State: vmTestState(t, schema, map[string]types.String{
				"id":   types.StringValue("uuid-123"),
				"name": types.StringValue("same-name"),
			}),
		}
		resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schema}}

		r.Update(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Update diagnostics: %v", resp.Diagnostics)
		}
		if mock.updateVMCalls != 0 {
			t.Fatalf("UpdateVM calls = %d, want 0", mock.updateVMCalls)
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			updateVMFunc: func(context.Context, string, vboxmanage.UpdateVMOptions) (*vboxmanage.VM, error) {
				return nil, errors.New("update failed")
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.UpdateRequest{
			Plan: vmTestPlan(t, schema, map[string]types.String{
				"name": types.StringValue("new-name"),
			}),
			State: vmTestState(t, schema, map[string]types.String{
				"id":   types.StringValue("uuid-123"),
				"name": types.StringValue("old-name"),
			}),
		}
		resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schema}}

		r.Update(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when update fails")
		}
	})
}

func TestVMResourceDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := vmTestSchema(t)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			deleteVMFunc: func(_ context.Context, id string) error {
				if id != "uuid-123" {
					t.Fatalf("DeleteVM id = %q, want %q", id, "uuid-123")
				}
				return nil
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.DeleteRequest{
			State: vmTestState(t, schema, map[string]types.String{
				"id":   types.StringValue("uuid-123"),
				"name": types.StringValue("test-vm"),
			}),
		}
		resp := &resource.DeleteResponse{}

		r.Delete(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Delete diagnostics: %v", resp.Diagnostics)
		}
		if mock.deleteVMCalls != 1 {
			t.Fatalf("DeleteVM calls = %d, want 1", mock.deleteVMCalls)
		}
	})

	t.Run("vm not found is ignored", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			deleteVMFunc: func(context.Context, string) error {
				return vboxmanage.ErrVMNotFound
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.DeleteRequest{
			State: vmTestState(t, schema, map[string]types.String{
				"id":   types.StringValue("uuid-123"),
				"name": types.StringValue("test-vm"),
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
			deleteVMFunc: func(context.Context, string) error {
				return errors.New("delete failed")
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.DeleteRequest{
			State: vmTestState(t, schema, map[string]types.String{
				"id":   types.StringValue("uuid-123"),
				"name": types.StringValue("test-vm"),
			}),
		}
		resp := &resource.DeleteResponse{}

		r.Delete(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when delete fails")
		}
	})
}
