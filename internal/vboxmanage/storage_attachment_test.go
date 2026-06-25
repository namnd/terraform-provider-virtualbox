// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const fakeStorageAttachmentScript = `#!/bin/sh
set -eu

STATE_DIR="$(dirname "$0")/state"
mkdir -p "$STATE_DIR"

vm_state_path() {
	echo "$STATE_DIR/$1"
}

attachments_file() {
	echo "$(vm_state_path "$1").attachments"
}

storage_indices_path() {
	echo "$(vm_state_path "$1").storage_indices"
}

storage_index_path() {
	echo "$(vm_state_path "$1").storage_$2"
}

case "$1" in
showvminfo)
	id="$2"
	shift 2
	machine_readable=false
	for arg in "$@"; do
		if [ "$arg" = "--machinereadable" ]; then
			machine_readable=true
		fi
	done
	if [ "$machine_readable" = true ]; then
		echo "name=\"vm-$id\""
		echo "UUID=\"$id\""
		indices_file="$(storage_indices_path "$id")"
		if [ -f "$indices_file" ]; then
			for idx in $(cat "$indices_file"); do
				base="$(storage_index_path "$id" "$idx")"
				if [ -f "${base}.name" ]; then
					echo "storagecontrollername$idx=\"$(cat "${base}.name")\""
				fi
			done
		fi
		attachments="$(attachments_file "$id")"
		if [ -f "$attachments" ]; then
			while IFS='|' read -r controller type port device medium; do
				[ -n "$controller" ] || continue
				echo "\"$controller-$port-$device\"=\"$medium\""
			done < "$attachments"
		fi
	fi
	exit 0
	;;
storagectl)
	id="$2"
	shift 2
	name=""
	add_type=""
	while [ $# -gt 0 ]; do
		case "$1" in
		--name) name="$2"; shift 2 ;;
		--add) add_type="$2"; shift 2 ;;
		*) shift ;;
		esac
	done
	idx=0
	indices_file="$(storage_indices_path "$id")"
	if [ -f "$indices_file" ]; then
		idx=$(wc -w < "$indices_file")
	fi
	if [ ! -f "$indices_file" ]; then
		echo "$idx" > "$indices_file"
	else
		echo "$idx" >> "$indices_file"
	fi
	base="$(storage_index_path "$id" "$idx")"
	echo "$name" > "${base}.name"
	echo "$add_type" > "${base}.bus"
	exit 0
	;;
storageattach)
	id="$2"
	shift 2
	storagectl=""
	port=""
	device=""
	medium=""
	type="hdd"
	while [ $# -gt 0 ]; do
		case "$1" in
		--storagectl) storagectl="$2"; shift 2 ;;
		--port) port="$2"; shift 2 ;;
		--device) device="$2"; shift 2 ;;
		--medium) medium="$2"; shift 2 ;;
		--type) type="$2"; shift 2 ;;
		*) shift ;;
		esac
	done
	attachments="$(attachments_file "$id")"
	if [ "$medium" = "none" ]; then
		if [ -f "$attachments" ]; then
			tmp="${attachments}.new"
			: > "$tmp"
			while IFS='|' read -r controller row_type row_port row_device row_medium; do
				if [ "$controller" = "$storagectl" ] && [ "$row_port" = "$port" ] && [ "$row_device" = "$device" ]; then
					continue
				fi
				echo "$controller|$row_type|$row_port|$row_device|$row_medium" >> "$tmp"
			done < "$attachments"
			mv "$tmp" "$attachments"
		fi
		exit 0
	fi
	if [ ! -f "$attachments" ]; then
		: > "$attachments"
	fi
	tmp="${attachments}.new"
	: > "$tmp"
	found=false
	if [ -f "$attachments" ]; then
		while IFS='|' read -r controller row_type row_port row_device row_medium; do
			if [ "$controller" = "$storagectl" ] && [ "$row_port" = "$port" ] && [ "$row_device" = "$device" ]; then
				echo "$storagectl|$type|$port|$device|$medium" >> "$tmp"
				found=true
			else
				echo "$controller|$row_type|$row_port|$row_device|$row_medium" >> "$tmp"
			fi
		done < "$attachments"
	fi
	if [ "$found" = false ]; then
		echo "$storagectl|$type|$port|$device|$medium" >> "$tmp"
	fi
	mv "$tmp" "$attachments"
	exit 0
	;;
*)
	echo "unknown command: $1" >&2
	exit 1
	;;
esac
`

func TestValidateStorageAttachment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		attachment StorageAttachment
		wantErr    string
	}{
		{
			name: "valid hdd attachment",
			attachment: StorageAttachment{
				VMID:           "uuid-123",
				ControllerName: "SATA Controller",
				Port:           0,
				Device:         0,
				Type:           StorageAttachmentTypeHDD,
				Medium:         "/data/boot.vdi",
			},
		},
		{
			name:       "empty vm id",
			attachment: StorageAttachment{ControllerName: "SATA Controller", Medium: "/data/boot.vdi"},
			wantErr:    "virtual machine id must not be empty",
		},
		{
			name:       "empty controller name",
			attachment: StorageAttachment{VMID: "uuid-123", Medium: "/data/boot.vdi"},
			wantErr:    "controller_name must not be empty",
		},
		{
			name: "invalid type",
			attachment: StorageAttachment{
				VMID:           "uuid-123",
				ControllerName: "SATA Controller",
				Medium:         "/data/boot.vdi",
				Type:           "invalid",
			},
			wantErr: "unsupported type",
		},
		{
			name: "empty medium",
			attachment: StorageAttachment{
				VMID:           "uuid-123",
				ControllerName: "SATA Controller",
			},
			wantErr: "medium must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateStorageAttachment(tt.attachment)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateStorageAttachment() error = %v, want nil", err)
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

func TestFormatParseStorageAttachmentID(t *testing.T) {
	t.Parallel()

	id := FormatStorageAttachmentID("uuid-123", "SATA Controller", 0, 1)
	vmID, controllerName, port, device, err := ParseStorageAttachmentID(id)
	if err != nil {
		t.Fatalf("ParseStorageAttachmentID() error: %v", err)
	}
	if vmID != "uuid-123" || controllerName != "SATA Controller" || port != 0 || device != 1 {
		t.Fatalf("parsed = %q %q %d %d, want uuid-123 SATA Controller 0 1", vmID, controllerName, port, device)
	}
}

func TestBuildStorageAttachArgs(t *testing.T) {
	t.Parallel()

	args, err := buildStorageAttachArgs("uuid-123", StorageAttachment{
		VMID:           "uuid-123",
		ControllerName: "SATA Controller",
		Port:           0,
		Device:         0,
		Type:           StorageAttachmentTypeHDD,
		Medium:         "/data/boot.vdi",
	})
	if err != nil {
		t.Fatalf("buildStorageAttachArgs() error: %v", err)
	}
	want := []string{
		"storageattach", "uuid-123",
		"--storagectl", "SATA Controller",
		"--port", "0",
		"--device", "0",
		"--type", StorageAttachmentTypeHDD,
		"--medium", "/data/boot.vdi",
	}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("buildStorageAttachArgs() = %v, want %v", args, want)
	}
}

func TestParseStorageAttachments(t *testing.T) {
	t.Parallel()

	t.Run("virtualbox machine readable format", func(t *testing.T) {
		t.Parallel()

		stdout := `"IDE Controller-0-0"="none"
"IDE Controller-1-0"="/home/namnguyen/Downloads/metal-amd64-v1.11.1.iso"
"IDE Controller-ImageUUID-1-0"="dfcbb86a-7bed-44fd-832b-ad6be775cb2f"
"IDE Controller-tempeject-1-0"="off"
`
		got := parseStorageAttachments(stdout)
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1; got %+v", len(got), got)
		}
		if got[0].ControllerName != "IDE Controller" || got[0].Port != 1 || got[0].Device != 0 {
			t.Fatalf("attachment = %+v, want IDE Controller port 1 device 0", got[0])
		}
		if got[0].Type != StorageAttachmentTypeDVDDrive {
			t.Fatalf("Type = %q, want %q", got[0].Type, StorageAttachmentTypeDVDDrive)
		}
	})

	stdout := `storagecontrollername0="SATA Controller"
"SATA Controller-0-0"="/data/boot.vdi"
"SATA Controller-1-0"="/data/install.iso"
"SATA Controller-ImageUUID-1-0"="uuid-for-install-iso"
`
	got := parseStorageAttachments(stdout)
	want := []StorageAttachment{
		{
			ControllerName: "SATA Controller",
			Port:           0,
			Device:         0,
			Type:           StorageAttachmentTypeHDD,
			Medium:         "/data/boot.vdi",
			MediumType:     StorageMediumTypeNormal,
		},
		{
			ControllerName: "SATA Controller",
			Port:           1,
			Device:         0,
			Type:           StorageAttachmentTypeDVDDrive,
			Medium:         "/data/install.iso",
			MediumType:     StorageMediumTypeNormal,
		},
	}
	if len(got) != len(want) {
		t.Fatalf("parseStorageAttachments() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseStorageAttachments()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseStorageAttachmentKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key            string
		wantController string
		wantPort       int
		wantDevice     int
		wantOK         bool
	}{
		{key: "IDE Controller-1-0", wantController: "IDE Controller", wantPort: 1, wantDevice: 0, wantOK: true},
		{key: "SATA Controller-0-0", wantController: "SATA Controller", wantPort: 0, wantDevice: 0, wantOK: true},
		{key: "IDE Controller-ImageUUID-1-0", wantOK: false},
		{key: "IDE Controller-tempeject-1-0", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()

			controller, port, device, ok := parseStorageAttachmentKey(tt.key)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if controller != tt.wantController || port != tt.wantPort || device != tt.wantDevice {
				t.Fatalf("parsed = %q %d %d, want %q %d %d", controller, port, device, tt.wantController, tt.wantPort, tt.wantDevice)
			}
		})
	}
}

func TestInferStorageAttachmentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		medium string
		want   string
	}{
		{medium: "/data/boot.vdi", want: StorageAttachmentTypeHDD},
		{medium: "/data/install.iso", want: StorageAttachmentTypeDVDDrive},
		{medium: "dfcbb86a-7bed-44fd-832b-ad6be775cb2f", want: StorageAttachmentTypeHDD},
	}

	for _, tt := range tests {
		t.Run(tt.medium, func(t *testing.T) {
			t.Parallel()

			if got := inferStorageAttachmentType(tt.medium); got != tt.want {
				t.Fatalf("inferStorageAttachmentType(%q) = %q, want %q", tt.medium, got, tt.want)
			}
		})
	}
}

func TestStorageAttachmentClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	client := newTestClient(t, fakeStorageAttachmentScript)

	_, _, err := client.RunWithOutput(ctx, "storagectl", "vm-1", "--name", "SATA Controller", "--add", "sata")
	if err != nil {
		t.Fatalf("storagectl setup error: %v", err)
	}

	t.Run("create and get attachment", func(t *testing.T) {
		attachment, err := client.CreateStorageAttachment(ctx, "vm-1", CreateStorageAttachmentOptions{
			ControllerName: "SATA Controller",
			Port:           0,
			Device:         0,
			Type:           StorageAttachmentTypeHDD,
			Medium:         "/data/boot.vdi",
		})
		if err != nil {
			t.Fatalf("CreateStorageAttachment() error: %v", err)
		}
		if attachment.Medium != "/data/boot.vdi" {
			t.Fatalf("attachment.Medium = %q, want %q", attachment.Medium, "/data/boot.vdi")
		}
		if attachment.Type != StorageAttachmentTypeHDD {
			t.Fatalf("attachment.Type = %q, want %q", attachment.Type, StorageAttachmentTypeHDD)
		}
	})

	t.Run("controller not found", func(t *testing.T) {
		_, err := client.CreateStorageAttachment(ctx, "vm-1", CreateStorageAttachmentOptions{
			ControllerName: "Missing Controller",
			Port:           0,
			Device:         0,
			Medium:         "/data/boot.vdi",
		})
		if err == nil || !strings.Contains(err.Error(), "storage controller") {
			t.Fatalf("CreateStorageAttachment() error = %v, want controller not found error", err)
		}
	})

	t.Run("update attachment medium", func(t *testing.T) {
		medium := "/data/new.vdi"
		attachment, err := client.UpdateStorageAttachment(ctx, "vm-1", "SATA Controller", 0, 0, UpdateStorageAttachmentOptions{
			Medium: &medium,
		})
		if err != nil {
			t.Fatalf("UpdateStorageAttachment() error: %v", err)
		}
		if attachment.Medium != "/data/new.vdi" {
			t.Fatalf("attachment.Medium = %q, want %q", attachment.Medium, "/data/new.vdi")
		}
	})

	t.Run("delete attachment", func(t *testing.T) {
		if err := client.DeleteStorageAttachment(ctx, "vm-1", "SATA Controller", 0, 0); err != nil {
			t.Fatalf("DeleteStorageAttachment() error: %v", err)
		}
		_, err := client.GetStorageAttachment(ctx, "vm-1", "SATA Controller", 0, 0)
		if err == nil || !strings.Contains(err.Error(), "storage attachment not found") {
			t.Fatalf("GetStorageAttachment() error = %v, want not found", err)
		}
	})
}

func fakeStorageAttachmentScriptWithCounter(stateDir string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

STATE_DIR=%q
COUNTER_FILE="$STATE_DIR/showvminfo_machinereadable_count"
mkdir -p "$STATE_DIR"

vm_state_path() {
	echo "$STATE_DIR/$1"
}

attachments_file() {
	echo "$(vm_state_path "$1").attachments"
}

storage_indices_path() {
	echo "$(vm_state_path "$1").storage_indices"
}

storage_index_path() {
	echo "$(vm_state_path "$1").storage_$2"
}

increment_showvminfo_counter() {
	count=0
	if [ -f "$COUNTER_FILE" ]; then
		count=$(cat "$COUNTER_FILE")
	fi
	count=$((count + 1))
	echo "$count" > "$COUNTER_FILE"
}

case "$1" in
showvminfo)
	id="$2"
	shift 2
	machine_readable=false
	for arg in "$@"; do
		if [ "$arg" = "--machinereadable" ]; then
			machine_readable=true
		fi
	done
	if [ "$machine_readable" = true ]; then
		increment_showvminfo_counter
		echo "name=\"vm-$id\""
		echo "UUID=\"$id\""
		indices_file="$(storage_indices_path "$id")"
		if [ -f "$indices_file" ]; then
			for idx in $(cat "$indices_file"); do
				base="$(storage_index_path "$id" "$idx")"
				if [ -f "${base}.name" ]; then
					echo "storagecontrollername$idx=\"$(cat "${base}.name")\""
				fi
			done
		fi
		attachments="$(attachments_file "$id")"
		if [ -f "$attachments" ]; then
			while IFS='|' read -r controller type port device medium; do
				[ -n "$controller" ] || continue
				echo "\"$controller-$port-$device\"=\"$medium\""
			done < "$attachments"
		fi
	fi
	exit 0
	;;
storagectl)
	id="$2"
	shift 2
	name=""
	while [ $# -gt 0 ]; do
		case "$1" in
		--name) name="$2"; shift 2 ;;
		--add) shift 2 ;;
		*) shift ;;
		esac
	done
	idx=0
	indices_file="$(storage_indices_path "$id")"
	if [ -f "$indices_file" ]; then
		idx=$(wc -w < "$indices_file")
	fi
	if [ ! -f "$indices_file" ]; then
		echo "$idx" > "$indices_file"
	else
		echo "$idx" >> "$indices_file"
	fi
	base="$(storage_index_path "$id" "$idx")"
	echo "$name" > "${base}.name"
	exit 0
	;;
storageattach)
	id="$2"
	shift 2
	storagectl=""
	port=""
	device=""
	medium=""
	type="hdd"
	while [ $# -gt 0 ]; do
		case "$1" in
		--storagectl) storagectl="$2"; shift 2 ;;
		--port) port="$2"; shift 2 ;;
		--device) device="$2"; shift 2 ;;
		--medium) medium="$2"; shift 2 ;;
		--type) type="$2"; shift 2 ;;
		*) shift ;;
		esac
	done
	attachments="$(attachments_file "$id")"
	if [ ! -f "$attachments" ]; then
		: > "$attachments"
	fi
	echo "$storagectl|$type|$port|$device|$medium" >> "$attachments"
	exit 0
	;;
*)
	echo "unknown command: $1" >&2
	exit 1
	;;
esac
`, stateDir)
}

func readShowvminfoMachineReadableCount(t *testing.T, counterFile string) int {
	t.Helper()

	data, err := os.ReadFile(counterFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read showvminfo counter: %v", err)
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse showvminfo counter %q: %v", string(data), err)
	}

	return count
}

func TestCreateStorageAttachment_ShowvminfoCallCount(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(t.TempDir(), "state")
	counterFile := filepath.Join(stateDir, "showvminfo_machinereadable_count")
	client := newTestClient(t, fakeStorageAttachmentScriptWithCounter(stateDir))
	ctx := context.Background()

	_, _, err := client.RunWithOutput(ctx, "storagectl", "vm-1", "--name", "SATA Controller", "--add", "sata")
	if err != nil {
		t.Fatalf("storagectl setup error: %v", err)
	}

	t.Run("create uses one showvminfo before attach and one after", func(t *testing.T) {
		attachment, err := client.CreateStorageAttachment(ctx, "vm-1", CreateStorageAttachmentOptions{
			ControllerName: "SATA Controller",
			Port:           0,
			Device:         0,
			Type:           StorageAttachmentTypeHDD,
			Medium:         "/data/boot.vdi",
		})
		if err != nil {
			t.Fatalf("CreateStorageAttachment() error: %v", err)
		}
		if attachment.Medium != "/data/boot.vdi" {
			t.Fatalf("attachment.Medium = %q, want %q", attachment.Medium, "/data/boot.vdi")
		}

		if got := readShowvminfoMachineReadableCount(t, counterFile); got != 2 {
			t.Fatalf("machine-readable showvminfo calls = %d, want 2 (1 before attach + 1 verify after attach)", got)
		}
	})

	t.Run("controller not found uses one showvminfo", func(t *testing.T) {
		if err := os.WriteFile(counterFile, []byte("0"), 0o644); err != nil {
			t.Fatalf("reset showvminfo counter: %v", err)
		}

		_, err := client.CreateStorageAttachment(ctx, "vm-1", CreateStorageAttachmentOptions{
			ControllerName: "Missing Controller",
			Port:           0,
			Device:         0,
			Medium:         "/data/boot.vdi",
		})
		if err == nil || !strings.Contains(err.Error(), "storage controller") {
			t.Fatalf("CreateStorageAttachment() error = %v, want controller not found error", err)
		}

		if got := readShowvminfoMachineReadableCount(t, counterFile); got != 1 {
			t.Fatalf("machine-readable showvminfo calls = %d, want 1 (validation only)", got)
		}
	})
}
