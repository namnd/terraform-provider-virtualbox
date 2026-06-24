// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"strings"
	"testing"
)

func TestValidateStorageController(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		controller StorageController
		wantErr    string
	}{
		{
			name: "ide controller",
			controller: StorageController{
				Name:       "IDE Controller",
				Type:       StorageBusIDE,
				Controller: StorageChipPIIX4,
			},
		},
		{
			name: "sata controller with options",
			controller: StorageController{
				Name:        "SATA Controller",
				Type:        StorageBusSATA,
				Controller:  StorageChipIntelAHCI,
				Bootable:    StorageBootableOn,
				HostIOCache: StorageHostIOCacheOn,
				PortCount:   2,
			},
		},
		{
			name: "controller without chip",
			controller: StorageController{
				Name: "IDE Controller",
				Type: StorageBusIDE,
			},
		},
		{
			name:       "empty name",
			controller: StorageController{Type: StorageBusIDE},
			wantErr:    "name must not be empty",
		},
		{
			name:       "empty type",
			controller: StorageController{Name: "IDE Controller"},
			wantErr:    "type must not be empty",
		},
		{
			name: "unsupported type",
			controller: StorageController{
				Name: "Bad Controller",
				Type: "invalid",
			},
			wantErr: "unsupported storage controller type",
		},
		{
			name: "invalid chip for type",
			controller: StorageController{
				Name:       "IDE Controller",
				Type:       StorageBusIDE,
				Controller: StorageChipIntelAHCI,
			},
			wantErr: `controller "IntelAHCI" is not valid for type "ide"`,
		},
		{
			name: "invalid bootable",
			controller: StorageController{
				Name:     "IDE Controller",
				Type:     StorageBusIDE,
				Bootable: "maybe",
			},
			wantErr: "unsupported bootable",
		},
		{
			name: "invalid hostiocache",
			controller: StorageController{
				Name:        "IDE Controller",
				Type:        StorageBusIDE,
				HostIOCache: "maybe",
			},
			wantErr: "unsupported hostiocache",
		},
		{
			name: "negative port count",
			controller: StorageController{
				Name:      "IDE Controller",
				Type:      StorageBusIDE,
				PortCount: -1,
			},
			wantErr: "portcount must be at least 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateStorageController(tt.controller)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateStorageController() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNormalizeStorageControllerChip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		chip string
		want string
	}{
		{chip: "AHCI", want: StorageChipIntelAHCI},
		{chip: "IntelAhci", want: StorageChipIntelAHCI},
		{chip: StorageChipPIIX4, want: StorageChipPIIX4},
		{chip: "  PIIX4  ", want: StorageChipPIIX4},
	}

	for _, tt := range tests {
		t.Run(tt.chip, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeStorageControllerChip(tt.chip); got != tt.want {
				t.Fatalf("NormalizeStorageControllerChip(%q) = %q, want %q", tt.chip, got, tt.want)
			}
		})
	}
}

func TestBusTypeFromChip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		chip string
		want string
	}{
		{chip: StorageChipPIIX4, want: StorageBusIDE},
		{chip: "AHCI", want: StorageBusSATA},
		{chip: StorageChipIntelAHCI, want: StorageBusSATA},
		{chip: StorageChipNVMe, want: StorageBusPCIe},
	}

	for _, tt := range tests {
		t.Run(tt.chip, func(t *testing.T) {
			t.Parallel()

			if got := BusTypeFromChip(tt.chip); got != tt.want {
				t.Fatalf("BusTypeFromChip(%q) = %q, want %q", tt.chip, got, tt.want)
			}
		})
	}
}

func TestNormalizeStorageBootable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  string
	}{
		{value: "", want: StorageBootableOn},
		{value: StorageBootableOn, want: StorageBootableOn},
		{value: "true", want: StorageBootableOn},
		{value: StorageBootableOff, want: StorageBootableOff},
		{value: "false", want: StorageBootableOff},
		{value: "maybe", want: "maybe"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeStorageBootable(tt.value); got != tt.want {
				t.Fatalf("NormalizeStorageBootable(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestNormalizeStorageHostIOCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  string
	}{
		{value: "", want: StorageHostIOCacheOff},
		{value: StorageHostIOCacheOff, want: StorageHostIOCacheOff},
		{value: "false", want: StorageHostIOCacheOff},
		{value: StorageHostIOCacheOn, want: StorageHostIOCacheOn},
		{value: "true", want: StorageHostIOCacheOn},
		{value: "maybe", want: "maybe"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeStorageHostIOCache(tt.value); got != tt.want {
				t.Fatalf("NormalizeStorageHostIOCache(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestBuildStorageCtlArgs(t *testing.T) {
	t.Parallel()

	t.Run("add ide controller", func(t *testing.T) {
		t.Parallel()

		args, err := buildStorageCtlArgs("uuid-123", StorageController{
			Name:        "IDE Controller",
			Type:        StorageBusIDE,
			Controller:  StorageChipPIIX4,
			HostIOCache: StorageHostIOCacheOn,
		}, true)
		if err != nil {
			t.Fatalf("buildStorageCtlArgs() error: %v", err)
		}
		want := []string{
			"storagectl", "uuid-123",
			"--name", "IDE Controller",
			"--add", StorageBusIDE,
			"--controller", StorageChipPIIX4,
			"--bootable", StorageBootableOn,
			"--hostiocache", StorageHostIOCacheOn,
		}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			t.Fatalf("buildStorageCtlArgs() = %v, want %v", args, want)
		}
	})

	t.Run("modify sata controller", func(t *testing.T) {
		t.Parallel()

		args, err := buildStorageCtlArgs("uuid-123", StorageController{
			Name:       "SATA Controller",
			Type:       StorageBusSATA,
			Controller: StorageChipIntelAHCI,
			Bootable:   StorageBootableOff,
			PortCount:  4,
		}, false)
		if err != nil {
			t.Fatalf("buildStorageCtlArgs() error: %v", err)
		}
		want := []string{
			"storagectl", "uuid-123",
			"--name", "SATA Controller",
			"--controller", StorageChipIntelAHCI,
			"--bootable", StorageBootableOff,
			"--hostiocache", StorageHostIOCacheOff,
			"--portcount", "4",
		}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			t.Fatalf("buildStorageCtlArgs() = %v, want %v", args, want)
		}
	})

	t.Run("invalid controller", func(t *testing.T) {
		t.Parallel()

		_, err := buildStorageCtlArgs("uuid-123", StorageController{
			Name: "IDE Controller",
			Type: "invalid",
		}, true)
		if err == nil {
			t.Fatal("expected error for invalid controller")
		}
	})
}

func TestStorageControllerNeedsUpdate(t *testing.T) {
	t.Parallel()

	current := StorageController{
		Controller:  StorageChipPIIX4,
		Bootable:    StorageBootableOn,
		HostIOCache: StorageHostIOCacheOff,
		PortCount:   2,
	}

	t.Run("no changes", func(t *testing.T) {
		t.Parallel()

		desired := current
		if storageControllerNeedsUpdate(current, desired) {
			t.Fatal("expected no update needed for identical controllers")
		}
	})

	t.Run("controller chip change", func(t *testing.T) {
		t.Parallel()

		desired := current
		desired.Controller = StorageChipPIIX3
		if !storageControllerNeedsUpdate(current, desired) {
			t.Fatal("expected update needed for controller chip change")
		}
	})

	t.Run("bootable change", func(t *testing.T) {
		t.Parallel()

		desired := current
		desired.Bootable = StorageBootableOff
		if !storageControllerNeedsUpdate(current, desired) {
			t.Fatal("expected update needed for bootable change")
		}
	})

	t.Run("hostiocache change", func(t *testing.T) {
		t.Parallel()

		desired := current
		desired.HostIOCache = StorageHostIOCacheOn
		if !storageControllerNeedsUpdate(current, desired) {
			t.Fatal("expected update needed for hostiocache change")
		}
	})

	t.Run("port count change", func(t *testing.T) {
		t.Parallel()

		desired := current
		desired.PortCount = 4
		if !storageControllerNeedsUpdate(current, desired) {
			t.Fatal("expected update needed for port count change")
		}
	})

	t.Run("normalizes ahci chip alias", func(t *testing.T) {
		t.Parallel()

		desired := current
		desired.Controller = "AHCI"
		current.Controller = StorageChipIntelAHCI
		if storageControllerNeedsUpdate(current, desired) {
			t.Fatal("expected no update needed for equivalent chip aliases")
		}
	})
}

func TestUpdateVMOptionsHasChangesStorageControllers(t *testing.T) {
	t.Parallel()

	controllers := []StorageController{{Name: "IDE Controller", Type: StorageBusIDE}}
	opts := UpdateVMOptions{StorageControllers: &controllers}
	if !opts.HasChanges() {
		t.Fatal("expected HasChanges() to be true when storage controllers are set")
	}
}
