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

func TestDiskResourceConfigureAcceptsVirtualBox(t *testing.T) {
	t.Parallel()

	mock := &mockVirtualBox{}
	r := &diskResource{}
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

func TestDiskResourceCreateUsesVirtualBox(t *testing.T) {
	t.Parallel()

	var createdPath string
	var createdSize int
	mock := &mockVirtualBox{
		createDiskFn: func(_ context.Context, opts vboxmanage.CreateDiskOptions) (*vboxmanage.Disk, error) {
			createdPath = opts.FilePath
			createdSize = opts.Size
			return &vboxmanage.Disk{
				UUID:     "34b3c171-5ebc-4f3b-bc30-73a7b1637987",
				FilePath: opts.FilePath,
				Size:     opts.Size,
				Format:   opts.Format,
				Variant:  opts.Variant,
			}, nil
		},
	}

	r := &diskResource{vbox: mock}

	disk, err := r.vbox.CreateDisk(context.Background(), vboxmanage.CreateDiskOptions{
		FilePath: "/tmp/test.vdi",
		Size:     2048,
		Format:   vboxmanage.DiskFormatVDI,
		Variant:  vboxmanage.DiskVariantStandard,
	})
	if err != nil {
		t.Fatalf("CreateDisk() error = %v", err)
	}
	if createdPath != "/tmp/test.vdi" {
		t.Fatalf("createdPath = %q, want %q", createdPath, "/tmp/test.vdi")
	}
	if createdSize != 2048 {
		t.Fatalf("createdSize = %d, want %d", createdSize, 2048)
	}
	if disk.UUID == "" {
		t.Fatal("expected disk UUID from mock")
	}
}

func TestDiskResourceUpdateUsesVirtualBox(t *testing.T) {
	t.Parallel()

	var resizedSize int
	mock := &mockVirtualBox{
		updateDiskFn: func(_ context.Context, _ string, opts vboxmanage.UpdateDiskOptions) (*vboxmanage.Disk, error) {
			if opts.Size != nil {
				resizedSize = *opts.Size
			}
			return &vboxmanage.Disk{
				UUID:     "34b3c171-5ebc-4f3b-bc30-73a7b1637987",
				FilePath: "/tmp/test.vdi",
				Size:     resizedSize,
				Format:   vboxmanage.DiskFormatVDI,
				Variant:  vboxmanage.DiskVariantStandard,
			}, nil
		},
	}

	r := &diskResource{vbox: mock}

	size := 4096
	disk, err := r.vbox.UpdateDisk(context.Background(), "/tmp/test.vdi", vboxmanage.UpdateDiskOptions{
		Size: &size,
	})
	if err != nil {
		t.Fatalf("UpdateDisk() error = %v", err)
	}
	if resizedSize != 4096 {
		t.Fatalf("resizedSize = %d, want %d", resizedSize, 4096)
	}
	if disk.Size != 4096 {
		t.Fatalf("disk.Size = %d, want %d", disk.Size, 4096)
	}
}

func TestDiskResourceDeleteUsesVirtualBox(t *testing.T) {
	t.Parallel()

	var deletedPath string
	mock := &mockVirtualBox{
		deleteDiskFn: func(_ context.Context, id string) error {
			deletedPath = id
			return nil
		},
	}

	r := &diskResource{vbox: mock}

	if err := r.vbox.DeleteDisk(context.Background(), "/tmp/test.vdi"); err != nil {
		t.Fatalf("DeleteDisk() error = %v", err)
	}
	if deletedPath != "/tmp/test.vdi" {
		t.Fatalf("deletedPath = %q, want %q", deletedPath, "/tmp/test.vdi")
	}
}

func TestMockVirtualBoxPropagatesDiskErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("boom")
	mock := &mockVirtualBox{
		getDiskFn: func(context.Context, string) (*vboxmanage.Disk, error) {
			return nil, want
		},
		deleteDiskFn: func(context.Context, string) error {
			return want
		},
	}

	if _, err := mock.GetDisk(context.Background(), "/tmp/test.vdi"); !errors.Is(err, want) {
		t.Fatalf("GetDisk() error = %v, want %v", err, want)
	}

	if err := mock.DeleteDisk(context.Background(), "/tmp/test.vdi"); !errors.Is(err, want) {
		t.Fatalf("DeleteDisk() error = %v, want %v", err, want)
	}
}
