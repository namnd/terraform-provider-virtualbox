// Copyright (c) HashiCorp, Inc.

package vboxmanage

import "context"

// VirtualBox is the domain interface used by the Terraform provider.
// Resources depend on this interface rather than the concrete Client.
type VirtualBox interface {
	Version(ctx context.Context) (string, error)
	CreateVM(ctx context.Context, name string, opts CreateVMOptions) (*VM, error)
	GetVM(ctx context.Context, id string) (*VM, error)
	UpdateVM(ctx context.Context, id string, opts UpdateVMOptions) (*VM, error)
	DeleteVM(ctx context.Context, id string) error
	CreateDisk(ctx context.Context, opts CreateDiskOptions) (*Disk, error)
	GetDisk(ctx context.Context, id string) (*Disk, error)
	UpdateDisk(ctx context.Context, id string, opts UpdateDiskOptions) (*Disk, error)
	DeleteDisk(ctx context.Context, id string) error
}
