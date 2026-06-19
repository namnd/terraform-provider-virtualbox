// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"errors"
	"testing"
)

func TestParseShowMediumInfoOutput(t *testing.T) {
	t.Parallel()

	stdout := `UUID:           34b3c171-5ebc-4f3b-bc30-73a7b1637987
Location:       /tmp/test-disk.vdi
Storage format: VDI
Format variant: dynamic default
Capacity:       2048 MBytes
Accessible:     yes
`

	disk, err := parseShowMediumInfoOutput(stdout)
	if err != nil {
		t.Fatalf("parseShowMediumInfoOutput() error = %v", err)
	}

	if disk.UUID != "34b3c171-5ebc-4f3b-bc30-73a7b1637987" {
		t.Fatalf("UUID = %q, want expected UUID", disk.UUID)
	}
	if disk.FilePath != "/tmp/test-disk.vdi" {
		t.Fatalf("FilePath = %q, want %q", disk.FilePath, "/tmp/test-disk.vdi")
	}
	if disk.Size != 2048 {
		t.Fatalf("Size = %d, want %d", disk.Size, 2048)
	}
	if disk.Format != "VDI" {
		t.Fatalf("Format = %q, want %q", disk.Format, "VDI")
	}
	if disk.Variant != DiskVariantStandard {
		t.Fatalf("Variant = %q, want %q", disk.Variant, DiskVariantStandard)
	}
	if !disk.Accessible {
		t.Fatal("expected disk to be accessible")
	}
}

func TestParseShowMediumInfoOutputFixedVariant(t *testing.T) {
	t.Parallel()

	stdout := `UUID:           34b3c171-5ebc-4f3b-bc30-73a7b1637987
Storage format: VMDK
Format variant: fixed default
Capacity:       1024 MBytes
`

	disk, err := parseShowMediumInfoOutput(stdout)
	if err != nil {
		t.Fatalf("parseShowMediumInfoOutput() error = %v", err)
	}
	if disk.Variant != DiskVariantFixed {
		t.Fatalf("Variant = %q, want %q", disk.Variant, DiskVariantFixed)
	}
}

func TestValidateCreateDiskOptions(t *testing.T) {
	t.Parallel()

	err := ValidateCreateDiskOptions(CreateDiskOptions{
		FilePath: "/tmp/test.vdi",
		Size:     1024,
		Format:   DiskFormatVDI,
		Variant:  DiskVariantStandard,
	})
	if err != nil {
		t.Fatalf("ValidateCreateDiskOptions() error = %v", err)
	}

	err = ValidateCreateDiskOptions(CreateDiskOptions{
		FilePath: "",
		Size:     1024,
	})
	if err == nil {
		t.Fatal("expected error for empty file path")
	}
}

func TestClassifyMediumError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stderr string
		want   error
	}{
		{
			name:   "already exists",
			stderr: "VBoxManage: error: VDI: cannot create image (VERR_ALREADY_EXISTS)",
			want:   ErrMediumAlreadyExists,
		},
		{
			name:   "not found",
			stderr: "VBoxManage: error: Could not find file for the medium '/tmp/missing.vdi' (VERR_FILE_NOT_FOUND)",
			want:   ErrMediumNotFound,
		},
		{
			name:   "unknown",
			stderr: "VBoxManage: error: something else",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyMediumError(tt.stderr)
			if !errors.Is(got, tt.want) {
				t.Fatalf("classifyMediumError() = %v, want %v", got, tt.want)
			}
		})
	}
}
