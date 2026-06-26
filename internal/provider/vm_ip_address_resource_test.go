// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func newTestVMIPAddressResource(t *testing.T, client vboxmanage.VirtualBox) *vmIPAddressResource {
	t.Helper()

	return &vmIPAddressResource{client: client}
}

func TestVMIPAddressResourceMetadata(t *testing.T) {
	t.Parallel()

	r := NewVMIPAddressResource()
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "virtualbox"}, resp)

	if resp.TypeName != "virtualbox_vm_ip_address" {
		t.Fatalf("TypeName = %q, want %q", resp.TypeName, "virtualbox_vm_ip_address")
	}
}

func TestVMIPAddressResourceConfigure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("nil provider data", func(t *testing.T) {
		t.Parallel()

		r := &vmIPAddressResource{}
		resp := &resource.ConfigureResponse{}
		r.Configure(ctx, resource.ConfigureRequest{}, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
	})

	t.Run("invalid provider data type", func(t *testing.T) {
		t.Parallel()

		r := &vmIPAddressResource{}
		resp := &resource.ConfigureResponse{}
		r.Configure(ctx, resource.ConfigureRequest{ProviderData: "invalid"}, resp)

		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error for invalid provider data type")
		}
	})

	t.Run("valid provider data", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{}
		r := &vmIPAddressResource{}
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

func TestVMIPAddressResourceCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := vmIPAddressTestSchema(t)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			getVMFunc: func(_ context.Context, id string) (*vboxmanage.VM, error) {
				if id != "uuid-vm-1" {
					t.Fatalf("GetVM id = %q, want %q", id, "uuid-vm-1")
				}
				return &vboxmanage.VM{
					UUID: id,
					NetworkAdapters: []vboxmanage.NetworkAdapter{
						{
							Type:          vboxmanage.NetworkTypeBridged,
							HostInterface: "eth0",
							MACAddress:    "08:00:27:AA:BB:CC",
						},
					},
				}, nil
			},
			getVMIPAddressFunc: func(_ context.Context, id string, opts vboxmanage.GetVMIPAddressOptions) (*string, error) {
				if id != "uuid-vm-1" {
					t.Fatalf("GetVMIPAddress id = %q, want %q", id, "uuid-vm-1")
				}
				if len(opts.NetworkAdapters) != 1 {
					t.Fatalf("len(NetworkAdapters) = %d, want 1", len(opts.NetworkAdapters))
				}
				if opts.NetworkAdapters[0].MACAddress != "08:00:27:AA:BB:CC" {
					t.Fatalf("MACAddress = %q, want %q", opts.NetworkAdapters[0].MACAddress, "08:00:27:AA:BB:CC")
				}
				ip := "192.168.1.42"
				return &ip, nil
			},
		}
		r := newTestVMIPAddressResource(t, mock)
		plan := vmIPAddressTestPlan(t, schema, vmIPAddressTestAttributeValues{
			Strings: map[string]types.String{
				"vm_id": types.StringValue("uuid-vm-1"),
			},
		})

		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}
		r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}

		state := vmIPAddressGetStateModel(t, ctx, resp.State)
		if state.VMID.ValueString() != "uuid-vm-1" {
			t.Fatalf("vm_id = %q, want %q", state.VMID.ValueString(), "uuid-vm-1")
		}
		if state.IP_Address.ValueString() != "192.168.1.42" {
			t.Fatalf("ip_address = %q, want %q", state.IP_Address.ValueString(), "192.168.1.42")
		}
		if state.LastUpdated.IsNull() || state.LastUpdated.IsUnknown() {
			t.Fatal("expected last_updated to be set")
		}
		if mock.getVMCalls != 1 {
			t.Fatalf("getVMCalls = %d, want 1", mock.getVMCalls)
		}
		if mock.getVMIPAddressCalls != 1 {
			t.Fatalf("getVMIPAddressCalls = %d, want 1", mock.getVMIPAddressCalls)
		}
	})

	t.Run("vm not found", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			getVMFunc: func(context.Context, string) (*vboxmanage.VM, error) {
				return nil, vboxmanage.ErrVMNotFound
			},
		}
		r := newTestVMIPAddressResource(t, mock)
		plan := vmIPAddressTestPlan(t, schema, vmIPAddressTestAttributeValues{
			Strings: map[string]types.String{
				"vm_id": types.StringValue("missing"),
			},
		})

		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}
		r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when VM is not found")
		}
		if mock.getVMIPAddressCalls != 0 {
			t.Fatalf("getVMIPAddressCalls = %d, want 0", mock.getVMIPAddressCalls)
		}
	})

	t.Run("applies default create timeout", func(t *testing.T) {
		t.Parallel()

		const deadlineSlack = 2 * time.Second

		mock := &mockVirtualBox{
			getVMFunc: func(context.Context, string) (*vboxmanage.VM, error) {
				return &vboxmanage.VM{
					NetworkAdapters: []vboxmanage.NetworkAdapter{
						{
							Type:       vboxmanage.NetworkTypeBridged,
							MACAddress: "08:00:27:AA:BB:CC",
						},
					},
				}, nil
			},
			getVMIPAddressFunc: func(ctx context.Context, _ string, _ vboxmanage.GetVMIPAddressOptions) (*string, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("expected context deadline")
				}

				remaining := time.Until(deadline)
				if remaining < vmIPAddressCreateTimeout-deadlineSlack || remaining > vmIPAddressCreateTimeout {
					t.Fatalf("remaining = %v, want ~%v", remaining, vmIPAddressCreateTimeout)
				}

				ip := "192.168.1.42"
				return &ip, nil
			},
		}
		r := newTestVMIPAddressResource(t, mock)
		plan := vmIPAddressTestPlan(t, schema, vmIPAddressTestAttributeValues{
			Strings: map[string]types.String{
				"vm_id": types.StringValue("uuid-vm-1"),
			},
		})

		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}
		r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
	})

	t.Run("applies configured create timeout", func(t *testing.T) {
		t.Parallel()

		const (
			configuredTimeout = 30 * time.Second
			deadlineSlack     = 2 * time.Second
		)

		mock := &mockVirtualBox{
			getVMFunc: func(context.Context, string) (*vboxmanage.VM, error) {
				return &vboxmanage.VM{
					NetworkAdapters: []vboxmanage.NetworkAdapter{
						{
							Type:       vboxmanage.NetworkTypeBridged,
							MACAddress: "08:00:27:AA:BB:CC",
						},
					},
				}, nil
			},
			getVMIPAddressFunc: func(ctx context.Context, _ string, _ vboxmanage.GetVMIPAddressOptions) (*string, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("expected context deadline")
				}

				remaining := time.Until(deadline)
				if remaining < configuredTimeout-deadlineSlack || remaining > configuredTimeout {
					t.Fatalf("remaining = %v, want ~%v", remaining, configuredTimeout)
				}

				ip := "192.168.1.42"
				return &ip, nil
			},
		}
		r := newTestVMIPAddressResource(t, mock)
		plan := vmIPAddressTestPlanFromModel(t, schema, vmIPAddressResourceModel{
			VMID:     types.StringValue("uuid-vm-1"),
			Timeouts: vmIPAddressTestTimeouts(t, "30s"),
		})

		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}
		r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
	})

	t.Run("ip lookup error", func(t *testing.T) {
		t.Parallel()

		mock := &mockVirtualBox{
			getVMIPAddressFunc: func(context.Context, string, vboxmanage.GetVMIPAddressOptions) (*string, error) {
				return nil, errors.New("arp lookup failed")
			},
		}
		r := newTestVMIPAddressResource(t, mock)
		plan := vmIPAddressTestPlan(t, schema, vmIPAddressTestAttributeValues{
			Strings: map[string]types.String{
				"vm_id": types.StringValue("uuid-vm-1"),
			},
		})

		resp := &resource.CreateResponse{State: tfsdk.State{Schema: schema}}
		r.Create(ctx, resource.CreateRequest{Plan: plan}, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error when IP lookup fails")
		}
		if mock.getVMCalls != 1 {
			t.Fatalf("getVMCalls = %d, want 1", mock.getVMCalls)
		}
		if mock.getVMIPAddressCalls != 1 {
			t.Fatalf("getVMIPAddressCalls = %d, want 1", mock.getVMIPAddressCalls)
		}
	})
}
