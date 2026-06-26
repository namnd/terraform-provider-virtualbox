// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeVMIPAddressVBoxManageScript = `#!/bin/sh
set -eu

STATE_DIR="$(dirname "$0")/state"
mkdir -p "$STATE_DIR"

vm_state_file() {
	echo "$STATE_DIR/$1.state"
}

vm_get_state() {
	path="$(vm_state_file "$1")"
	if [ -f "$path" ]; then
		cat "$path"
	else
		echo "poweroff"
	fi
}

vm_set_state() {
	echo "$2" > "$(vm_state_file "$1")"
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
		echo "VMState=\"$(vm_get_state "$id")\""
	fi
	exit 0
	;;
startvm)
	id="$2"
	vm_set_state "$id" "running"
	exit 0
	;;
controlvm)
	id="$2"
	action="$3"
	if [ "$action" = "poweroff" ]; then
		vm_set_state "$id" "poweroff"
	fi
	exit 0
	;;
discardstate)
	id="$2"
	vm_set_state "$id" "poweroff"
	exit 0
	;;
*)
	echo "unknown command: $1" >&2
	exit 1
	;;
esac
`

func TestParseIPFromARPOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		output  string
		want    string
		wantErr error
	}{
		{
			name:   "linux arp format",
			output: "? (192.168.1.100) at 08:00:27:aa:bb:cc [ether] on eth0",
			want:   "192.168.1.100",
		},
		{
			name:   "bsd arp format",
			output: "host.example.com (10.0.0.5) at 08:00:27:aa:bb:cc on en0 ifscope [ethernet]",
			want:   "10.0.0.5",
		},
		{
			name:    "no ip address",
			output:  "no matching entries",
			wantErr: errIPNotFoundInARP,
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: errIPNotFoundInARP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseIPFromARPOutput(tt.output)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("parseIPFromARPOutput() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseIPFromARPOutput() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("parseIPFromARPOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "08:00:27:aa:bb:cc", want: "'08:00:27:aa:bb:cc'"},
		{input: "it's", want: "'it'\\''s'"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			if got := shellQuote(tt.input); got != tt.want {
				t.Fatalf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsGrepNoMatch(t *testing.T) {
	t.Parallel()

	t.Run("grep exit code 1", func(t *testing.T) {
		t.Parallel()

		err := exec.Command("sh", "-c", "false").Run()
		if !isGrepNoMatch(err) {
			t.Fatal("expected grep no-match error")
		}
	})

	t.Run("other exit code", func(t *testing.T) {
		t.Parallel()

		err := exec.Command("sh", "-c", "exit 2").Run()
		if isGrepNoMatch(err) {
			t.Fatal("expected non-grep error")
		}
	})
}

func TestVMStateHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state             string
		wantRunning       bool
		wantStartable     bool
		wantPoweredOff    bool
		wantNeedsPowerOff bool
	}{
		{state: "running", wantRunning: true, wantNeedsPowerOff: true},
		{state: "poweroff", wantStartable: true, wantPoweredOff: true},
		{state: "aborted", wantStartable: true, wantPoweredOff: true},
		{state: "saved", wantStartable: true, wantNeedsPowerOff: true},
		{state: "paused", wantNeedsPowerOff: true},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			t.Parallel()

			if got := isVMRunning(tt.state); got != tt.wantRunning {
				t.Fatalf("isVMRunning(%q) = %v, want %v", tt.state, got, tt.wantRunning)
			}
			if got := isVMStartable(tt.state); got != tt.wantStartable {
				t.Fatalf("isVMStartable(%q) = %v, want %v", tt.state, got, tt.wantStartable)
			}
			if got := isVMPoweredOff(tt.state); got != tt.wantPoweredOff {
				t.Fatalf("isVMPoweredOff(%q) = %v, want %v", tt.state, got, tt.wantPoweredOff)
			}
			if got := vmStateNeedsPowerOff(tt.state); got != tt.wantNeedsPowerOff {
				t.Fatalf("vmStateNeedsPowerOff(%q) = %v, want %v", tt.state, got, tt.wantNeedsPowerOff)
			}
		})
	}
}

func TestLookupIPByMAC(t *testing.T) {
	t.Run("empty mac", func(t *testing.T) {
		t.Parallel()

		_, err := lookupIPByMAC(context.Background(), "")
		if err == nil || !strings.Contains(err.Error(), "mac address must not be empty") {
			t.Fatalf("lookupIPByMAC() error = %v, want mac address must not be empty", err)
		}
	})

	t.Run("success with fake arp", func(t *testing.T) {
		binDir := t.TempDir()
		arpPath := filepath.Join(binDir, "arp")
		if err := os.WriteFile(arpPath, []byte(`#!/bin/sh
if [ "$1" = "-a" ]; then
  echo "? (192.168.1.55) at 08:00:27:de:ad:be [ether] on eth0"
fi
`), 0o755); err != nil {
			t.Fatalf("failed to write fake arp script: %v", err)
		}

		ctx := context.Background()
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		ip, err := lookupIPByMAC(ctx, "08:00:27:de:ad:be")
		if err != nil {
			t.Fatalf("lookupIPByMAC() error = %v", err)
		}
		if ip != "192.168.1.55" {
			t.Fatalf("lookupIPByMAC() = %q, want %q", ip, "192.168.1.55")
		}
	})

	t.Run("mac not found", func(t *testing.T) {
		binDir := t.TempDir()
		arpPath := filepath.Join(binDir, "arp")
		if err := os.WriteFile(arpPath, []byte(`#!/bin/sh
if [ "$1" = "-a" ]; then
  echo "? (192.168.1.55) at 08:00:27:de:ad:be [ether] on eth0"
fi
`), 0o755); err != nil {
			t.Fatalf("failed to write fake arp script: %v", err)
		}

		ctx := context.Background()
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		_, err := lookupIPByMAC(ctx, "08:00:27:00:00:01")
		if !errors.Is(err, errIPNotFoundInARP) {
			t.Fatalf("lookupIPByMAC() error = %v, want %v", err, errIPNotFoundInARP)
		}
	})
}

func testVMIPLookupContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	return ctx
}

func TestGetVMIPAddress(t *testing.T) {
	t.Run("empty id", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, fakeVMIPAddressVBoxManageScript)
		_, err := client.GetVMIPAddress(testVMIPLookupContext(t), "  ", GetVMIPAddressOptions{})
		if err == nil || !strings.Contains(err.Error(), "virtual machine id must not be empty") {
			t.Fatalf("GetVMIPAddress() error = %v, want empty id error", err)
		}
	})

	t.Run("missing context deadline", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, fakeVMIPAddressVBoxManageScript)
		_, err := client.GetVMIPAddress(context.Background(), "uuid-test-vm", GetVMIPAddressOptions{})
		if !errors.Is(err, ErrContextDeadlineRequired) {
			t.Fatalf("GetVMIPAddress() error = %v, want %v", err, ErrContextDeadlineRequired)
		}
	})

	t.Run("success starts vm and powers off", func(t *testing.T) {
		binDir := t.TempDir()
		arpPath := filepath.Join(binDir, "arp")
		if err := os.WriteFile(arpPath, []byte(`#!/bin/sh
if [ "$1" = "-a" ]; then
  echo "? (10.0.0.42) at 08:00:27:11:22:33 [ether] on eth0"
fi
`), 0o755); err != nil {
			t.Fatalf("failed to write fake arp script: %v", err)
		}
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		client := newTestClient(t, fakeVMIPAddressVBoxManageScript)
		ip, err := client.GetVMIPAddress(testVMIPLookupContext(t), "uuid-test-vm", GetVMIPAddressOptions{
			NetworkAdapters: []NetworkAdapter{
				{
					Type:       NetworkTypeBridged,
					MACAddress: "08:00:27:11:22:33",
				},
			},
		})
		if err != nil {
			t.Fatalf("GetVMIPAddress() error = %v", err)
		}
		if ip == nil || *ip != "10.0.0.42" {
			t.Fatalf("GetVMIPAddress() = %v, want 10.0.0.42", ip)
		}

		state, err := client.vmState(context.Background(), "uuid-test-vm")
		if err != nil {
			t.Fatalf("vmState() error = %v", err)
		}
		if state != "poweroff" {
			t.Fatalf("vmState() = %q, want %q", state, "poweroff")
		}
	})

	t.Run("times out waiting for arp entry", func(t *testing.T) {
		binDir := t.TempDir()
		arpPath := filepath.Join(binDir, "arp")
		if err := os.WriteFile(arpPath, []byte(`#!/bin/sh
if [ "$1" = "-a" ]; then
  echo "? (192.168.1.55) at 08:00:27:de:ad:be [ether] on eth0"
fi
`), 0o755); err != nil {
			t.Fatalf("failed to write fake arp script: %v", err)
		}
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		client := newTestClient(t, fakeVMIPAddressVBoxManageScript)
		stateDir := filepath.Join(filepath.Dir(client.binary), "state")
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			t.Fatalf("failed to create state dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(stateDir, "uuid-timeout-vm.state"), []byte("running"), 0o644); err != nil {
			t.Fatalf("failed to seed vm state: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := client.GetVMIPAddress(ctx, "uuid-timeout-vm", GetVMIPAddressOptions{
			NetworkAdapters: []NetworkAdapter{
				{
					Type:       NetworkTypeBridged,
					MACAddress: "08:00:27:99:88:77",
				},
			},
		})
		if err == nil {
			t.Fatal("GetVMIPAddress() error = nil, want timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "timed out waiting for arp entry") {
			t.Fatalf("GetVMIPAddress() error = %v, want timeout error", err)
		}
	})

	t.Run("leaves already running vm running", func(t *testing.T) {
		binDir := t.TempDir()
		arpPath := filepath.Join(binDir, "arp")
		if err := os.WriteFile(arpPath, []byte(`#!/bin/sh
if [ "$1" = "-a" ]; then
  echo "? (172.16.0.8) at 08:00:27:44:55:66 [ether] on eth0"
fi
`), 0o755); err != nil {
			t.Fatalf("failed to write fake arp script: %v", err)
		}
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		client := newTestClient(t, fakeVMIPAddressVBoxManageScript)
		stateDir := filepath.Join(filepath.Dir(client.binary), "state")
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			t.Fatalf("failed to create state dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(stateDir, "uuid-running-vm.state"), []byte("running"), 0o644); err != nil {
			t.Fatalf("failed to seed vm state: %v", err)
		}

		ip, err := client.GetVMIPAddress(testVMIPLookupContext(t), "uuid-running-vm", GetVMIPAddressOptions{
			NetworkAdapters: []NetworkAdapter{
				{
					Type:       NetworkTypeBridged,
					MACAddress: "08:00:27:44:55:66",
				},
			},
		})
		if err != nil {
			t.Fatalf("GetVMIPAddress() error = %v", err)
		}
		if ip == nil || *ip != "172.16.0.8" {
			t.Fatalf("GetVMIPAddress() = %v, want 172.16.0.8", ip)
		}

		state, err := client.vmState(context.Background(), "uuid-running-vm")
		if err != nil {
			t.Fatalf("vmState() error = %v", err)
		}
		if state != "running" {
			t.Fatalf("vmState() = %q, want %q", state, "running")
		}
	})
}
