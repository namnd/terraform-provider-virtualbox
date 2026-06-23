// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const fakeDiskVBoxManageScript = `#!/bin/sh
set -eu

STATE_DIR="$(dirname "$0")/state"
mkdir -p "$STATE_DIR"

disk_key() {
	echo "$1" | tr '/' '_'
}

disk_find_key() {
	id="$1"
	for key_file in "$STATE_DIR"/*.uuid; do
		[ -f "$key_file" ] || continue
		key="$(basename "$key_file" .uuid)"
		stored_uuid="$(cat "$key_file")"
		stored_loc="$(cat "$STATE_DIR/$key.location")"
		if [ "$id" = "$stored_uuid" ] || [ "$id" = "$stored_loc" ]; then
			echo "$key"
			return 0
		fi
	done
	return 1
}

case "$1" in
createmedium)
	shift
	if [ "$1" != "disk" ]; then
		echo "unknown subcommand: $1" >&2
		exit 1
	fi
	shift
	filepath=""
	size=""
	format="VDI"
	variant="Standard"
	while [ $# -gt 0 ]; do
		case "$1" in
		--filename)
			filepath="$2"
			shift 2
			;;
		--size)
			size="$2"
			shift 2
			;;
		--format)
			format="$2"
			shift 2
			;;
		--variant)
			variant="$2"
			shift 2
			;;
		*)
			shift
			;;
		esac
	done
	key="$(disk_key "$filepath")"
	if [ -f "$STATE_DIR/$key.uuid" ]; then
		echo "VBoxManage: error: medium already exists (VERR_ALREADY_EXISTS)" >&2
		exit 1
	fi
	echo "uuid-for-$key" > "$STATE_DIR/$key.uuid"
	echo "$filepath" > "$STATE_DIR/$key.location"
	echo "$size" > "$STATE_DIR/$key.size"
	echo "$format" > "$STATE_DIR/$key.format"
	echo "$variant" > "$STATE_DIR/$key.variant"
	exit 0
	;;
showmediuminfo)
	shift
	if [ "$1" != "disk" ]; then
		echo "unknown subcommand: $1" >&2
		exit 1
	fi
	shift
	id="$1"
	if ! key="$(disk_find_key "$id")"; then
		echo "VBoxManage: error: Could not find file for the medium" >&2
		exit 1
	fi
	uuid="$(cat "$STATE_DIR/$key.uuid")"
	location="$(cat "$STATE_DIR/$key.location")"
	size="$(cat "$STATE_DIR/$key.size")"
	format="$(cat "$STATE_DIR/$key.format")"
	variant="$(cat "$STATE_DIR/$key.variant")"
	echo "UUID: $uuid"
	echo "Location: $location"
	echo "Storage format: $format"
	if [ "$variant" = "Fixed" ]; then
		echo "Format variant: fixed size"
	else
		echo "Format variant: dynamic (default)"
	fi
	echo "Capacity: $size MBytes"
	exit 0
	;;
modifymedium)
	shift
	if [ "$1" != "disk" ]; then
		echo "unknown subcommand: $1" >&2
		exit 1
	fi
	shift
	id="$1"
	shift
	if ! key="$(disk_find_key "$id")"; then
		echo "VBoxManage: error: Could not find a medium" >&2
		exit 1
	fi
	while [ $# -gt 0 ]; do
		case "$1" in
		--resize)
			echo "$2" > "$STATE_DIR/$key.size"
			shift 2
			;;
		*)
			shift
			;;
		esac
	done
	exit 0
	;;
closemedium)
	shift
	if [ "$1" != "disk" ]; then
		echo "unknown subcommand: $1" >&2
		exit 1
	fi
	shift
	id="$1"
	shift
	delete=false
	for arg in "$@"; do
		if [ "$arg" = "--delete" ]; then
			delete=true
		fi
	done
	if ! key="$(disk_find_key "$id")"; then
		echo "VBoxManage: error: Could not find file for the medium" >&2
		exit 1
	fi
	if [ "$delete" = true ]; then
		rm -f "$STATE_DIR/$key.uuid" "$STATE_DIR/$key.location" "$STATE_DIR/$key.size" "$STATE_DIR/$key.format" "$STATE_DIR/$key.variant"
	fi
	exit 0
	;;
*)
	echo "unknown command: $1" >&2
	exit 1
	;;
esac
`

func TestParseShowMediumInfoOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		stdout  string
		want    *Disk
		wantErr string
	}{
		{
			name: "parses standard dynamic disk",
			stdout: `UUID: abc-def-123
Location: /tmp/test.vdi
Storage format: VDI
Format variant: dynamic (default)
Capacity: 2048 MBytes
`,
			want: &Disk{
				UUID:     "abc-def-123",
				FilePath: "/tmp/test.vdi",
				Size:     2048,
				Format:   DiskFormatVDI,
				Variant:  DiskVariantStandard,
			},
		},
		{
			name: "parses fixed disk using logical size",
			stdout: `UUID: abc-def-456
Location: /tmp/fixed.vmdk
Storage format: VMDK
Format variant: fixed size
Logical size: 4096 MBytes
`,
			want: &Disk{
				UUID:     "abc-def-456",
				FilePath: "/tmp/fixed.vmdk",
				Size:     4096,
				Format:   DiskFormatVMDK,
				Variant:  DiskVariantFixed,
			},
		},
		{
			name:    "missing uuid",
			stdout:  `Location: /tmp/test.vdi`,
			wantErr: "UUID was not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseShowMediumInfoOutput(tt.stdout)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.UUID != tt.want.UUID ||
				got.FilePath != tt.want.FilePath ||
				got.Size != tt.want.Size ||
				got.Format != tt.want.Format ||
				got.Variant != tt.want.Variant {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseDiskVariant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "dynamic (default)", want: DiskVariantStandard},
		{input: "fixed size", want: DiskVariantFixed},
		{input: "Fixed", want: DiskVariantFixed},
		{input: "", want: DiskVariantStandard},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			if got := parseDiskVariant(tt.input); got != tt.want {
				t.Fatalf("parseDiskVariant(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
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
			stderr: "VBoxManage: error: medium already exists (VERR_ALREADY_EXISTS)",
			want:   ErrMediumAlreadyExists,
		},
		{
			name:   "not found by file",
			stderr: "VBoxManage: error: Could not find file for the medium",
			want:   ErrMediumNotFound,
		},
		{
			name:   "not found generic",
			stderr: "VBoxManage: error: Could not find a medium",
			want:   ErrMediumNotFound,
		},
		{
			name:   "unknown error",
			stderr: "VBoxManage: error: something else went wrong",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyMediumError(tt.stderr)
			if !errors.Is(got, tt.want) {
				t.Fatalf("classifyMediumError(%q) = %v, want %v", tt.stderr, got, tt.want)
			}
		})
	}
}

func TestCreateDisk(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := newTestClient(t, fakeDiskVBoxManageScript)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		disk, err := client.CreateDisk(ctx, CreateDiskOptions{
			FilePath: "/tmp/test.vdi",
			Size:     2048,
		})
		if err != nil {
			t.Fatalf("CreateDisk() error: %v", err)
		}
		if disk.UUID != "uuid-for-_tmp_test.vdi" {
			t.Fatalf("CreateDisk() UUID = %q, want %q", disk.UUID, "uuid-for-_tmp_test.vdi")
		}
		if disk.FilePath != "/tmp/test.vdi" {
			t.Fatalf("CreateDisk() FilePath = %q, want %q", disk.FilePath, "/tmp/test.vdi")
		}
		if disk.Size != 2048 {
			t.Fatalf("CreateDisk() Size = %d, want 2048", disk.Size)
		}
		if disk.Format != DiskFormatVDI {
			t.Fatalf("CreateDisk() Format = %q, want %q", disk.Format, DiskFormatVDI)
		}
		if disk.Variant != DiskVariantStandard {
			t.Fatalf("CreateDisk() Variant = %q, want %q", disk.Variant, DiskVariantStandard)
		}
	})

	t.Run("success with custom format and variant", func(t *testing.T) {
		t.Parallel()

		disk, err := client.CreateDisk(ctx, CreateDiskOptions{
			FilePath: "/tmp/fixed.vmdk",
			Size:     4096,
			Format:   DiskFormatVMDK,
			Variant:  DiskVariantFixed,
		})
		if err != nil {
			t.Fatalf("CreateDisk() error: %v", err)
		}
		if disk.Format != DiskFormatVMDK {
			t.Fatalf("CreateDisk() Format = %q, want %q", disk.Format, DiskFormatVMDK)
		}
		if disk.Variant != DiskVariantFixed {
			t.Fatalf("CreateDisk() Variant = %q, want %q", disk.Variant, DiskVariantFixed)
		}
	})

	t.Run("empty file path", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateDisk(ctx, CreateDiskOptions{Size: 1024})
		if err == nil || !strings.Contains(err.Error(), "file path must not be empty") {
			t.Fatalf("CreateDisk() error = %v, want empty file path validation error", err)
		}
	})

	t.Run("invalid size", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateDisk(ctx, CreateDiskOptions{
			FilePath: "/tmp/invalid.vdi",
			Size:     0,
		})
		if err == nil || !strings.Contains(err.Error(), "size must be at least 1 MiB") {
			t.Fatalf("CreateDisk() error = %v, want size validation error", err)
		}
	})

	t.Run("unsupported format", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateDisk(ctx, CreateDiskOptions{
			FilePath: "/tmp/invalid.vdi",
			Size:     1024,
			Format:   "RAW",
		})
		if err == nil || !strings.Contains(err.Error(), "unsupported disk format") {
			t.Fatalf("CreateDisk() error = %v, want format validation error", err)
		}
	})

	t.Run("unsupported variant", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateDisk(ctx, CreateDiskOptions{
			FilePath: "/tmp/invalid.vdi",
			Size:     1024,
			Variant:  "Split",
		})
		if err == nil || !strings.Contains(err.Error(), "unsupported disk variant") {
			t.Fatalf("CreateDisk() error = %v, want variant validation error", err)
		}
	})

	t.Run("already exists", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateDisk(ctx, CreateDiskOptions{
			FilePath: "/tmp/exists.vdi",
			Size:     1024,
		})
		if err != nil {
			t.Fatalf("CreateDisk() error: %v", err)
		}

		_, err = client.CreateDisk(ctx, CreateDiskOptions{
			FilePath: "/tmp/exists.vdi",
			Size:     1024,
		})
		if !errors.Is(err, ErrMediumAlreadyExists) {
			t.Fatalf("CreateDisk() error = %v, want ErrMediumAlreadyExists", err)
		}
	})
}

func TestGetDisk(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := newTestClient(t, fakeDiskVBoxManageScript)

	t.Run("success by file path", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateDisk(ctx, CreateDiskOptions{
			FilePath: "/tmp/get.vdi",
			Size:     1024,
		})
		if err != nil {
			t.Fatalf("CreateDisk() error: %v", err)
		}

		disk, err := client.GetDisk(ctx, "/tmp/get.vdi")
		if err != nil {
			t.Fatalf("GetDisk() error: %v", err)
		}
		if disk.FilePath != "/tmp/get.vdi" || disk.Size != 1024 {
			t.Fatalf("GetDisk() = %+v, want file_path=/tmp/get.vdi size=1024", disk)
		}
	})

	t.Run("success by uuid", func(t *testing.T) {
		t.Parallel()

		created, err := client.CreateDisk(ctx, CreateDiskOptions{
			FilePath: "/tmp/by-uuid.vdi",
			Size:     2048,
		})
		if err != nil {
			t.Fatalf("CreateDisk() error: %v", err)
		}

		disk, err := client.GetDisk(ctx, created.UUID)
		if err != nil {
			t.Fatalf("GetDisk() error: %v", err)
		}
		if disk.UUID != created.UUID || disk.Size != 2048 {
			t.Fatalf("GetDisk() = %+v, want uuid=%s size=2048", disk, created.UUID)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetDisk(ctx, "")
		if err == nil || !strings.Contains(err.Error(), "id must not be empty") {
			t.Fatalf("GetDisk() error = %v, want empty id validation error", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		_, err := client.GetDisk(ctx, "/tmp/missing.vdi")
		if !errors.Is(err, ErrMediumNotFound) {
			t.Fatalf("GetDisk() error = %v, want ErrMediumNotFound", err)
		}
	})
}

func TestUpdateDisk(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := newTestClient(t, fakeDiskVBoxManageScript)

	t.Run("resizes disk", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateDisk(ctx, CreateDiskOptions{
			FilePath: "/tmp/resize.vdi",
			Size:     1024,
		})
		if err != nil {
			t.Fatalf("CreateDisk() error: %v", err)
		}

		size := 4096
		disk, err := client.UpdateDisk(ctx, "/tmp/resize.vdi", UpdateDiskOptions{Size: &size})
		if err != nil {
			t.Fatalf("UpdateDisk() error: %v", err)
		}
		if disk.Size != 4096 {
			t.Fatalf("UpdateDisk() Size = %d, want 4096", disk.Size)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		t.Parallel()

		size := 2048
		_, err := client.UpdateDisk(ctx, "", UpdateDiskOptions{Size: &size})
		if err == nil || !strings.Contains(err.Error(), "id must not be empty") {
			t.Fatalf("UpdateDisk() error = %v, want empty id validation error", err)
		}
	})

	t.Run("no changes", func(t *testing.T) {
		t.Parallel()

		_, err := client.UpdateDisk(ctx, "/tmp/resize.vdi", UpdateDiskOptions{})
		if err == nil || !strings.Contains(err.Error(), "at least one disk setting must be provided") {
			t.Fatalf("UpdateDisk() error = %v, want no changes validation error", err)
		}
	})

	t.Run("invalid size", func(t *testing.T) {
		t.Parallel()

		size := 0
		_, err := client.UpdateDisk(ctx, "/tmp/resize.vdi", UpdateDiskOptions{Size: &size})
		if err == nil || !strings.Contains(err.Error(), "size must be at least 1 MiB") {
			t.Fatalf("UpdateDisk() error = %v, want size validation error", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		size := 2048
		_, err := client.UpdateDisk(ctx, "/tmp/missing.vdi", UpdateDiskOptions{Size: &size})
		if !errors.Is(err, ErrMediumNotFound) {
			t.Fatalf("UpdateDisk() error = %v, want ErrMediumNotFound", err)
		}
	})
}

func TestDeleteDisk(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := newTestClient(t, fakeDiskVBoxManageScript)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		_, err := client.CreateDisk(ctx, CreateDiskOptions{
			FilePath: "/tmp/delete.vdi",
			Size:     1024,
		})
		if err != nil {
			t.Fatalf("CreateDisk() error: %v", err)
		}

		if err := client.DeleteDisk(ctx, "/tmp/delete.vdi"); err != nil {
			t.Fatalf("DeleteDisk() error: %v", err)
		}

		_, err = client.GetDisk(ctx, "/tmp/delete.vdi")
		if !errors.Is(err, ErrMediumNotFound) {
			t.Fatalf("GetDisk() after delete error = %v, want ErrMediumNotFound", err)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		t.Parallel()

		err := client.DeleteDisk(ctx, "")
		if err == nil || !strings.Contains(err.Error(), "id must not be empty") {
			t.Fatalf("DeleteDisk() error = %v, want empty id validation error", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		err := client.DeleteDisk(ctx, "/tmp/missing.vdi")
		if !errors.Is(err, ErrMediumNotFound) {
			t.Fatalf("DeleteDisk() error = %v, want ErrMediumNotFound", err)
		}
	})
}
