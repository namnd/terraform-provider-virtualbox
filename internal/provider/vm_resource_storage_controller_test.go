// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func TestStorageControllersFromModel(t *testing.T) {
	t.Parallel()

	t.Run("valid ide controller", func(t *testing.T) {
		t.Parallel()

		controllers, diags := storageControllersFromModel([]storageControllerModel{
			{
				Name:        types.StringValue("IDE Controller"),
				Type:        types.StringValue("ide"),
				Controller:  types.StringValue("PIIX4"),
				HostIOCache: types.BoolValue(true),
			},
		})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if len(controllers) != 1 {
			t.Fatalf("len(controllers) = %d, want 1", len(controllers))
		}
		if controllers[0].Type != "ide" {
			t.Fatalf("controllers[0].Type = %q, want %q", controllers[0].Type, "ide")
		}
		if controllers[0].Controller != "PIIX4" {
			t.Fatalf("controllers[0].Controller = %q, want %q", controllers[0].Controller, "PIIX4")
		}
		if controllers[0].HostIOCache != vboxmanage.StorageHostIOCacheOn {
			t.Fatalf("controllers[0].HostIOCache = %q, want %q", controllers[0].HostIOCache, vboxmanage.StorageHostIOCacheOn)
		}
	})

	t.Run("valid sata controller with port count", func(t *testing.T) {
		t.Parallel()

		controllers, diags := storageControllersFromModel([]storageControllerModel{
			{
				Name:       types.StringValue("SATA Controller"),
				Type:       types.StringValue("sata"),
				Controller: types.StringValue("IntelAHCI"),
				Bootable:   types.BoolValue(true),
				PortCount:  types.Int64Value(2),
			},
		})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if controllers[0].Bootable != vboxmanage.StorageBootableOn {
			t.Fatalf("controllers[0].Bootable = %q, want %q", controllers[0].Bootable, vboxmanage.StorageBootableOn)
		}
		if controllers[0].PortCount != 2 {
			t.Fatalf("controllers[0].PortCount = %d, want 2", controllers[0].PortCount)
		}
	})

	t.Run("invalid chip for type", func(t *testing.T) {
		t.Parallel()

		_, diags := storageControllersFromModel([]storageControllerModel{
			{
				Name:       types.StringValue("IDE Controller"),
				Type:       types.StringValue("ide"),
				Controller: types.StringValue("IntelAHCI"),
			},
		})
		if !diags.HasError() {
			t.Fatal("expected diagnostics error for invalid controller chip")
		}
	})

	t.Run("reports index for invalid controller", func(t *testing.T) {
		t.Parallel()

		_, diags := storageControllersFromModel([]storageControllerModel{
			{
				Name: types.StringValue("IDE Controller"),
				Type: types.StringValue("ide"),
			},
			{
				Name: types.StringValue("Bad Controller"),
				Type: types.StringValue("invalid"),
			},
		})
		if !diags.HasError() {
			t.Fatal("expected diagnostics error for second controller")
		}
		if diags.Errors()[0].Summary() != "Invalid storage controller" {
			t.Fatalf("error summary = %q, want %q", diags.Errors()[0].Summary(), "Invalid storage controller")
		}
		if diags.Errors()[0].Detail() != `storage_controller[1]: unsupported storage controller type "invalid", must be floppy, ide, pcie, sas, sata, scsi, or usb` {
			t.Fatalf("error detail = %q", diags.Errors()[0].Detail())
		}
	})
}

func TestStorageControllersToModel(t *testing.T) {
	t.Parallel()

	t.Run("ide controller with host io cache", func(t *testing.T) {
		t.Parallel()

		models := storageControllersToModel([]vboxmanage.StorageController{
			{
				Name:        "IDE Controller",
				Type:        "ide",
				Controller:  "PIIX4",
				HostIOCache: vboxmanage.StorageHostIOCacheOn,
			},
		})
		if len(models) != 1 {
			t.Fatalf("len(models) = %d, want 1", len(models))
		}
		if models[0].HostIOCache.ValueBool() != true {
			t.Fatal("expected host_io_cache to be true")
		}
		if !models[0].PortCount.IsNull() {
			t.Fatal("expected port_count to be null when unset")
		}
	})

	t.Run("normalizes ahci chip alias", func(t *testing.T) {
		t.Parallel()

		models := storageControllersToModel([]vboxmanage.StorageController{
			{
				Name:       "SATA Controller",
				Type:       "sata",
				Controller: "AHCI",
				Bootable:   vboxmanage.StorageBootableOn,
				PortCount:  1,
			},
		})
		if models[0].Controller.ValueString() != "IntelAHCI" {
			t.Fatalf("models[0].Controller = %q, want %q", models[0].Controller.ValueString(), "IntelAHCI")
		}
		if !models[0].Bootable.ValueBool() {
			t.Fatal("expected bootable to be true")
		}
		if models[0].PortCount.ValueInt64() != 1 {
			t.Fatalf("models[0].PortCount = %d, want 1", models[0].PortCount.ValueInt64())
		}
	})
}

func TestStorageControllersModelEqual(t *testing.T) {
	t.Parallel()

	base := []storageControllerModel{
		{
			Name:        types.StringValue("IDE Controller"),
			Type:        types.StringValue("ide"),
			Controller:  types.StringValue("PIIX4"),
			Bootable:    types.BoolNull(),
			HostIOCache: types.BoolValue(true),
			PortCount:   types.Int64Null(),
		},
		{
			Name:        types.StringValue("SATA Controller"),
			Type:        types.StringValue("sata"),
			Controller:  types.StringValue("IntelAHCI"),
			Bootable:    types.BoolValue(true),
			HostIOCache: types.BoolNull(),
			PortCount:   types.Int64Value(1),
		},
	}

	t.Run("equal controllers", func(t *testing.T) {
		t.Parallel()

		other := []storageControllerModel{
			{
				Name:        types.StringValue("IDE Controller"),
				Type:        types.StringValue("ide"),
				Controller:  types.StringValue("PIIX4"),
				Bootable:    types.BoolNull(),
				HostIOCache: types.BoolValue(true),
				PortCount:   types.Int64Null(),
			},
			{
				Name:        types.StringValue("SATA Controller"),
				Type:        types.StringValue("sata"),
				Controller:  types.StringValue("IntelAHCI"),
				Bootable:    types.BoolValue(true),
				HostIOCache: types.BoolNull(),
				PortCount:   types.Int64Value(1),
			},
		}
		if !storageControllersModelEqual(base, other) {
			t.Fatal("expected controllers to be equal")
		}
	})

	t.Run("different lengths", func(t *testing.T) {
		t.Parallel()

		if storageControllersModelEqual(base, base[:1]) {
			t.Fatal("expected controllers with different lengths to be unequal")
		}
	})

	t.Run("different type", func(t *testing.T) {
		t.Parallel()

		other := []storageControllerModel{
			{
				Name:        types.StringValue("IDE Controller"),
				Type:        types.StringValue("sata"),
				Controller:  types.StringValue("PIIX4"),
				Bootable:    types.BoolNull(),
				HostIOCache: types.BoolValue(true),
				PortCount:   types.Int64Null(),
			},
		}
		if storageControllersModelEqual(base[:1], other) {
			t.Fatal("expected controllers with different types to be unequal")
		}
	})

	t.Run("different port count", func(t *testing.T) {
		t.Parallel()

		other := []storageControllerModel{
			{
				Name:        types.StringValue("SATA Controller"),
				Type:        types.StringValue("sata"),
				Controller:  types.StringValue("IntelAHCI"),
				Bootable:    types.BoolValue(true),
				HostIOCache: types.BoolNull(),
				PortCount:   types.Int64Value(2),
			},
		}
		if storageControllersModelEqual(base[1:], other) {
			t.Fatal("expected controllers with different port counts to be unequal")
		}
	})

	t.Run("different host io cache", func(t *testing.T) {
		t.Parallel()

		other := []storageControllerModel{
			{
				Name:        types.StringValue("IDE Controller"),
				Type:        types.StringValue("ide"),
				Controller:  types.StringValue("PIIX4"),
				Bootable:    types.BoolNull(),
				HostIOCache: types.BoolValue(false),
				PortCount:   types.Int64Null(),
			},
		}
		if storageControllersModelEqual(base[:1], other) {
			t.Fatal("expected controllers with different host_io_cache to be unequal")
		}
	})
}
