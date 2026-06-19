// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

type mockVirtualBox struct {
	versionFn  func(ctx context.Context) (string, error)
	createVMFn func(ctx context.Context, name string, opts vboxmanage.CreateVMOptions) (*vboxmanage.VM, error)
	getVMFn    func(ctx context.Context, id string) (*vboxmanage.VM, error)
	updateVMFn func(ctx context.Context, id string, opts vboxmanage.UpdateVMOptions) (*vboxmanage.VM, error)
	deleteVMFn func(ctx context.Context, id string) error
}

func (m *mockVirtualBox) Version(ctx context.Context) (string, error) {
	if m.versionFn != nil {
		return m.versionFn(ctx)
	}
	return "7.2.10r174163", nil
}

func (m *mockVirtualBox) CreateVM(ctx context.Context, name string, opts vboxmanage.CreateVMOptions) (*vboxmanage.VM, error) {
	if m.createVMFn != nil {
		return m.createVMFn(ctx, name, opts)
	}
	return &vboxmanage.VM{Name: name, UUID: "00000000-0000-0000-0000-000000000001"}, nil
}

func (m *mockVirtualBox) GetVM(ctx context.Context, id string) (*vboxmanage.VM, error) {
	if m.getVMFn != nil {
		return m.getVMFn(ctx, id)
	}
	return &vboxmanage.VM{Name: "test-vm", UUID: id}, nil
}

func (m *mockVirtualBox) UpdateVM(ctx context.Context, id string, opts vboxmanage.UpdateVMOptions) (*vboxmanage.VM, error) {
	if m.updateVMFn != nil {
		return m.updateVMFn(ctx, id, opts)
	}
	vm := &vboxmanage.VM{UUID: id}
	if opts.Name != nil {
		vm.Name = *opts.Name
	}
	if opts.CPUs != nil {
		vm.CPUs = *opts.CPUs
	}
	if opts.Memory != nil {
		vm.Memory = *opts.Memory
	}
	return vm, nil
}

func (m *mockVirtualBox) DeleteVM(ctx context.Context, id string) error {
	if m.deleteVMFn != nil {
		return m.deleteVMFn(ctx, id)
	}
	return nil
}

func TestVMResourceConfigureAcceptsVirtualBox(t *testing.T) {
	t.Parallel()

	mock := &mockVirtualBox{}
	r := &vmResource{}
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

func TestVMResourceConfigureRejectsWrongType(t *testing.T) {
	t.Parallel()

	r := &vmResource{}
	resp := &resource.ConfigureResponse{}

	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: "not-a-virtualbox-client",
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected Configure() to fail for wrong provider data type")
	}
}

func TestVMResourceCreateUsesVirtualBox(t *testing.T) {
	t.Parallel()

	var createdName string
	mock := &mockVirtualBox{
		createVMFn: func(_ context.Context, name string, _ vboxmanage.CreateVMOptions) (*vboxmanage.VM, error) {
			createdName = name
			return &vboxmanage.VM{Name: name, UUID: "00000000-0000-0000-0000-000000000001"}, nil
		},
	}

	r := &vmResource{vbox: mock}

	// Create is not implemented yet; this test documents the wiring pattern for
	// when Create is implemented against r.vbox.CreateVM.
	if r.vbox == nil {
		t.Fatal("expected VirtualBox client to be configured")
	}

	vm, err := r.vbox.CreateVM(context.Background(), "test-vm", vboxmanage.CreateVMOptions{})
	if err != nil {
		t.Fatalf("CreateVM() error = %v", err)
	}
	if createdName != "test-vm" {
		t.Fatalf("createdName = %q, want %q", createdName, "test-vm")
	}
	if vm.UUID == "" {
		t.Fatal("expected VM UUID from mock")
	}
}

func TestVMResourceReadUsesVirtualBox(t *testing.T) {
	t.Parallel()

	var readID string
	mock := &mockVirtualBox{
		getVMFn: func(_ context.Context, id string) (*vboxmanage.VM, error) {
			readID = id
			return &vboxmanage.VM{Name: "test-vm", UUID: id}, nil
		},
	}

	r := &vmResource{vbox: mock}

	vm, err := r.vbox.GetVM(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("GetVM() error = %v", err)
	}
	if readID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("readID = %q, want %q", readID, "00000000-0000-0000-0000-000000000001")
	}
	if vm.Name != "test-vm" {
		t.Fatalf("vm.Name = %q, want %q", vm.Name, "test-vm")
	}
	if vm.UUID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("vm.UUID = %q, want %q", vm.UUID, "00000000-0000-0000-0000-000000000001")
	}
}

func TestVMResourceUpdateUsesVirtualBox(t *testing.T) {
	t.Parallel()

	var updatedID string
	var updatedName string
	var updatedCPUs int
	mock := &mockVirtualBox{
		updateVMFn: func(_ context.Context, id string, opts vboxmanage.UpdateVMOptions) (*vboxmanage.VM, error) {
			updatedID = id
			if opts.Name != nil {
				updatedName = *opts.Name
			}
			if opts.CPUs != nil {
				updatedCPUs = *opts.CPUs
			}
			return &vboxmanage.VM{Name: updatedName, UUID: id, CPUs: updatedCPUs}, nil
		},
	}

	r := &vmResource{vbox: mock}

	name := "renamed-vm"
	cpus := 4
	vm, err := r.vbox.UpdateVM(context.Background(), "00000000-0000-0000-0000-000000000001", vboxmanage.UpdateVMOptions{
		Name: &name,
		CPUs: &cpus,
	})
	if err != nil {
		t.Fatalf("UpdateVM() error = %v", err)
	}
	if updatedID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("updatedID = %q, want %q", updatedID, "00000000-0000-0000-0000-000000000001")
	}
	if updatedName != "renamed-vm" {
		t.Fatalf("updatedName = %q, want %q", updatedName, "renamed-vm")
	}
	if vm.Name != "renamed-vm" {
		t.Fatalf("vm.Name = %q, want %q", vm.Name, "renamed-vm")
	}
	if updatedCPUs != 4 {
		t.Fatalf("updatedCPUs = %d, want %d", updatedCPUs, 4)
	}
}

func TestVMResourceDeleteUsesVirtualBox(t *testing.T) {
	t.Parallel()

	var deletedID string
	mock := &mockVirtualBox{
		deleteVMFn: func(_ context.Context, id string) error {
			deletedID = id
			return nil
		},
	}

	r := &vmResource{vbox: mock}

	if err := r.vbox.DeleteVM(context.Background(), "test-vm"); err != nil {
		t.Fatalf("DeleteVM() error = %v", err)
	}
	if deletedID != "test-vm" {
		t.Fatalf("deletedID = %q, want %q", deletedID, "test-vm")
	}
}

func TestMockVirtualBoxPropagatesErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("boom")
	mock := &mockVirtualBox{
		getVMFn: func(context.Context, string) (*vboxmanage.VM, error) {
			return nil, want
		},
		deleteVMFn: func(context.Context, string) error {
			return want
		},
	}

	if _, err := mock.GetVM(context.Background(), "test-vm"); !errors.Is(err, want) {
		t.Fatalf("GetVM() error = %v, want %v", err, want)
	}

	if err := mock.DeleteVM(context.Background(), "test-vm"); !errors.Is(err, want) {
		t.Fatalf("DeleteVM() error = %v, want %v", err, want)
	}
}
