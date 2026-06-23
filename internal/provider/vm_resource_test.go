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
	return &vboxmanage.VM{
		Name:            name,
		UUID:            "uuid-" + name,
		CPUs:            opts.CPUs,
		Memory:          opts.Memory,
		NetworkAdapters: opts.NetworkAdapters,
	}, nil
}

func (m *mockVirtualBox) GetVM(ctx context.Context, id string) (*vboxmanage.VM, error) {
	m.getVMCalls++
	if m.getVMFunc != nil {
		return m.getVMFunc(ctx, id)
	}
	return &vboxmanage.VM{Name: "vm-" + id, UUID: id, CPUs: 1, Memory: 1024}, nil
}

func (m *mockVirtualBox) UpdateVM(ctx context.Context, id string, opts vboxmanage.UpdateVMOptions) (*vboxmanage.VM, error) {
	m.updateVMCalls++
	if m.updateVMFunc != nil {
		return m.updateVMFunc(ctx, id, opts)
	}

	vm := &vboxmanage.VM{UUID: id, CPUs: 1, Memory: 1024}
	if opts.Name != nil {
		vm.Name = *opts.Name
	}
	if opts.CPUs != nil {
		vm.CPUs = *opts.CPUs
	}
	if opts.Memory != nil {
		vm.Memory = *opts.Memory
	}
	if opts.NetworkAdapters != nil {
		vm.NetworkAdapters = *opts.NetworkAdapters
	}
	return vm, nil
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
				if opts.CPUs != 2 {
					t.Fatalf("CreateVM CPUs = %d, want 2", opts.CPUs)
				}
				if opts.Memory != 2048 {
					t.Fatalf("CreateVM Memory = %d, want 2048", opts.Memory)
				}
				return &vboxmanage.VM{Name: name, UUID: "uuid-123", CPUs: 2, Memory: 2048}, nil
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.CreateRequest{
			Plan: vmTestPlan(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"name":    types.StringValue("test-vm"),
					"os_type": types.StringValue("Linux_64"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(2),
					"memory": types.Int64Value(2048),
				},
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
		if state.CPUs.ValueInt64() != 2 {
			t.Fatalf("state.CPUs = %d, want 2", state.CPUs.ValueInt64())
		}
		if state.Memory.ValueInt64() != 2048 {
			t.Fatalf("state.Memory = %d, want 2048", state.Memory.ValueInt64())
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
			Plan: vmTestPlan(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"name":    types.StringValue("test-vm"),
					"os_type": types.StringValue("Linux_64"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
			}),
		}
		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}

		r.Create(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when create fails")
		}
	})

	t.Run("creates vm with network adapters", func(t *testing.T) {
		t.Parallel()

		networkAdapters := []networkAdapterModel{
			{
				Type:            types.StringValue("nat"),
				HostInterface:   types.StringNull(),
				PromiscuousMode: types.StringValue("deny"),
			},
			{
				Type:            types.StringValue("bridged"),
				HostInterface:   types.StringValue("eth0"),
				PromiscuousMode: types.StringValue("allow-all"),
			},
		}

		mock := &mockVirtualBox{
			createVMFunc: func(_ context.Context, name string, opts vboxmanage.CreateVMOptions) (*vboxmanage.VM, error) {
				if len(opts.NetworkAdapters) != 2 {
					t.Fatalf("CreateVM NetworkAdapters len = %d, want 2", len(opts.NetworkAdapters))
				}
				if opts.NetworkAdapters[0].Type != "nat" {
					t.Fatalf("CreateVM NetworkAdapters[0].Type = %q, want %q", opts.NetworkAdapters[0].Type, "nat")
				}
				if opts.NetworkAdapters[1].Type != "bridged" || opts.NetworkAdapters[1].HostInterface != "eth0" {
					t.Fatalf("CreateVM NetworkAdapters[1] = %+v, want bridged on eth0", opts.NetworkAdapters[1])
				}
				return &vboxmanage.VM{
					Name:            name,
					UUID:            "uuid-123",
					CPUs:            opts.CPUs,
					Memory:          opts.Memory,
					NetworkAdapters: opts.NetworkAdapters,
				}, nil
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.CreateRequest{
			Plan: vmTestPlan(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"name":    types.StringValue("test-vm"),
					"os_type": types.StringValue("Linux_64"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
				NetworkAdapters: &networkAdapters,
			}),
		}
		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}

		r.Create(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Create diagnostics: %v", resp.Diagnostics)
		}

		state := vmGetStateModel(t, ctx, resp.State)
		if len(state.NetworkAdapters) != 2 {
			t.Fatalf("state.NetworkAdapters len = %d, want 2", len(state.NetworkAdapters))
		}
		if state.NetworkAdapters[0].Type.ValueString() != "nat" {
			t.Fatalf("state.NetworkAdapters[0].Type = %q, want %q", state.NetworkAdapters[0].Type.ValueString(), "nat")
		}
		if state.NetworkAdapters[1].HostInterface.ValueString() != "eth0" {
			t.Fatalf("state.NetworkAdapters[1].HostInterface = %q, want %q", state.NetworkAdapters[1].HostInterface.ValueString(), "eth0")
		}
	})

	t.Run("invalid network adapter", func(t *testing.T) {
		t.Parallel()

		networkAdapters := []networkAdapterModel{
			{Type: types.StringValue("bridged")},
		}

		r := newTestVMResource(t, &mockVirtualBox{})

		req := resource.CreateRequest{
			Plan: vmTestPlan(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"name":    types.StringValue("test-vm"),
					"os_type": types.StringValue("Linux_64"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
				NetworkAdapters: &networkAdapters,
			}),
		}
		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}

		r.Create(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error for invalid network adapter")
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
				return &vboxmanage.VM{Name: "updated-name", UUID: "uuid-123", CPUs: 4, Memory: 4096}, nil
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.ReadRequest{
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("old-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
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
		if state.CPUs.ValueInt64() != 4 {
			t.Fatalf("state.CPUs = %d, want 4", state.CPUs.ValueInt64())
		}
		if state.Memory.ValueInt64() != 4096 {
			t.Fatalf("state.Memory = %d, want 4096", state.Memory.ValueInt64())
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
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("test-vm"),
				},
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
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("test-vm"),
				},
			}),
		}
		resp := &resource.ReadResponse{State: tfsdk.State{Schema: schema}}

		r.Read(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when read fails")
		}
	})

	t.Run("populates network adapters", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			getVMFunc: func(_ context.Context, id string) (*vboxmanage.VM, error) {
				return &vboxmanage.VM{
					Name:   "test-vm",
					UUID:   id,
					CPUs:   1,
					Memory: 1024,
					NetworkAdapters: []vboxmanage.NetworkAdapter{
						{Type: "nat"},
						{
							Type:            "bridged",
							HostInterface:   "wlan0",
							PromiscuousMode: "allow-vms",
						},
					},
				}, nil
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.ReadRequest{
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("test-vm"),
				},
			}),
		}
		resp := &resource.ReadResponse{State: tfsdk.State{Schema: schema}}

		r.Read(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Read diagnostics: %v", resp.Diagnostics)
		}

		state := vmGetStateModel(t, ctx, resp.State)
		if len(state.NetworkAdapters) != 2 {
			t.Fatalf("state.NetworkAdapters len = %d, want 2", len(state.NetworkAdapters))
		}
		if state.NetworkAdapters[0].Type.ValueString() != "nat" {
			t.Fatalf("state.NetworkAdapters[0].Type = %q, want %q", state.NetworkAdapters[0].Type.ValueString(), "nat")
		}
		if state.NetworkAdapters[1].HostInterface.ValueString() != "wlan0" {
			t.Fatalf("state.NetworkAdapters[1].HostInterface = %q, want %q", state.NetworkAdapters[1].HostInterface.ValueString(), "wlan0")
		}
		if state.NetworkAdapters[1].PromiscuousMode.ValueString() != "allow-vms" {
			t.Fatalf("state.NetworkAdapters[1].PromiscuousMode = %q, want %q", state.NetworkAdapters[1].PromiscuousMode.ValueString(), "allow-vms")
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
				if opts.Name == nil || *opts.Name != "new-name" {
					t.Fatalf("UpdateVM name = %v, want %q", opts.Name, "new-name")
				}
				return &vboxmanage.VM{Name: "new-name", UUID: "uuid-123", CPUs: 1, Memory: 1024}, nil
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.UpdateRequest{
			Plan: vmTestPlan(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"name": types.StringValue("new-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
			}),
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("old-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
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

	t.Run("updates cpus and memory", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			updateVMFunc: func(_ context.Context, id string, opts vboxmanage.UpdateVMOptions) (*vboxmanage.VM, error) {
				if opts.CPUs == nil || *opts.CPUs != 4 {
					t.Fatalf("UpdateVM CPUs = %v, want 4", opts.CPUs)
				}
				if opts.Memory == nil || *opts.Memory != 4096 {
					t.Fatalf("UpdateVM Memory = %v, want 4096", opts.Memory)
				}
				if opts.Name != nil {
					t.Fatal("expected name not to be updated")
				}
				return &vboxmanage.VM{Name: "same-name", UUID: id, CPUs: 4, Memory: 4096}, nil
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.UpdateRequest{
			Plan: vmTestPlan(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"name": types.StringValue("same-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(4),
					"memory": types.Int64Value(4096),
				},
			}),
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("same-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
			}),
		}
		resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schema}}

		r.Update(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Update diagnostics: %v", resp.Diagnostics)
		}

		state := vmGetStateModel(t, ctx, resp.State)
		if state.CPUs.ValueInt64() != 4 {
			t.Fatalf("state.CPUs = %d, want 4", state.CPUs.ValueInt64())
		}
		if state.Memory.ValueInt64() != 4096 {
			t.Fatalf("state.Memory = %d, want 4096", state.Memory.ValueInt64())
		}
		if mock.updateVMCalls != 1 {
			t.Fatalf("UpdateVM calls = %d, want 1", mock.updateVMCalls)
		}
	})

	t.Run("skips update when unchanged", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{}
		r := newTestVMResource(t, mock)

		req := resource.UpdateRequest{
			Plan: vmTestPlan(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"name": types.StringValue("same-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(2),
					"memory": types.Int64Value(2048),
				},
			}),
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("same-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(2),
					"memory": types.Int64Value(2048),
				},
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
			Plan: vmTestPlan(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"name": types.StringValue("new-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
			}),
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("old-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
			}),
		}
		resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schema}}

		r.Update(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when update fails")
		}
	})

	t.Run("updates network adapters", func(t *testing.T) {
		t.Parallel()

		planAdapters := []networkAdapterModel{
			{
				Type:            types.StringValue("bridged"),
				HostInterface:   types.StringValue("eth0"),
				PromiscuousMode: types.StringValue("allow-all"),
			},
		}
		stateAdapters := []networkAdapterModel{
			{
				Type:            types.StringValue("nat"),
				HostInterface:   types.StringNull(),
				PromiscuousMode: types.StringValue("deny"),
			},
		}

		mock := &mockVirtualBox{
			updateVMFunc: func(_ context.Context, id string, opts vboxmanage.UpdateVMOptions) (*vboxmanage.VM, error) {
				if opts.NetworkAdapters == nil {
					t.Fatal("expected network adapters to be updated")
				}
				if len(*opts.NetworkAdapters) != 1 {
					t.Fatalf("UpdateVM NetworkAdapters len = %d, want 1", len(*opts.NetworkAdapters))
				}
				if (*opts.NetworkAdapters)[0].Type != "bridged" || (*opts.NetworkAdapters)[0].HostInterface != "eth0" {
					t.Fatalf("UpdateVM NetworkAdapters[0] = %+v, want bridged on eth0", (*opts.NetworkAdapters)[0])
				}
				return &vboxmanage.VM{
					Name:            "same-name",
					UUID:            id,
					CPUs:            1,
					Memory:          1024,
					NetworkAdapters: *opts.NetworkAdapters,
				}, nil
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.UpdateRequest{
			Plan: vmTestPlan(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"name": types.StringValue("same-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
				NetworkAdapters: &planAdapters,
			}),
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("same-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
				NetworkAdapters: &stateAdapters,
			}),
		}
		resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schema}}

		r.Update(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Update diagnostics: %v", resp.Diagnostics)
		}

		state := vmGetStateModel(t, ctx, resp.State)
		if len(state.NetworkAdapters) != 1 {
			t.Fatalf("state.NetworkAdapters len = %d, want 1", len(state.NetworkAdapters))
		}
		if state.NetworkAdapters[0].Type.ValueString() != "bridged" {
			t.Fatalf("state.NetworkAdapters[0].Type = %q, want %q", state.NetworkAdapters[0].Type.ValueString(), "bridged")
		}
		if mock.updateVMCalls != 1 {
			t.Fatalf("UpdateVM calls = %d, want 1", mock.updateVMCalls)
		}
	})

	t.Run("skips update when network adapters unchanged", func(t *testing.T) {
		t.Parallel()

		networkAdapters := []networkAdapterModel{
			{
				Type:            types.StringValue("nat"),
				HostInterface:   types.StringNull(),
				PromiscuousMode: types.StringValue("deny"),
			},
		}

		mock := &mockVirtualBox{}
		r := newTestVMResource(t, mock)

		req := resource.UpdateRequest{
			Plan: vmTestPlan(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"name": types.StringValue("same-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
				NetworkAdapters: &networkAdapters,
			}),
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("same-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
				NetworkAdapters: &networkAdapters,
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

	t.Run("invalid network adapter", func(t *testing.T) {
		t.Parallel()

		planAdapters := []networkAdapterModel{
			{Type: types.StringValue("invalid")},
		}
		stateAdapters := []networkAdapterModel{
			{
				Type:            types.StringValue("nat"),
				HostInterface:   types.StringNull(),
				PromiscuousMode: types.StringValue("deny"),
			},
		}

		r := newTestVMResource(t, &mockVirtualBox{})

		req := resource.UpdateRequest{
			Plan: vmTestPlan(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"name": types.StringValue("same-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
				NetworkAdapters: &planAdapters,
			}),
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("same-name"),
				},
				Int64s: map[string]types.Int64{
					"cpus":   types.Int64Value(1),
					"memory": types.Int64Value(1024),
				},
				NetworkAdapters: &stateAdapters,
			}),
		}
		resp := &resource.UpdateResponse{State: tfsdk.State{Schema: schema}}

		r.Update(ctx, req, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error for invalid network adapter")
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
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("test-vm"),
				},
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
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("test-vm"),
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
			deleteVMFunc: func(context.Context, string) error {
				return errors.New("delete failed")
			},
		}
		r := newTestVMResource(t, mock)

		req := resource.DeleteRequest{
			State: vmTestState(t, schema, vmTestAttributeValues{
				Strings: map[string]types.String{
					"id":   types.StringValue("uuid-123"),
					"name": types.StringValue("test-vm"),
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
