// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"errors"
	"os/exec"
	"testing"
)

func TestParseIPFromARPOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		output  string
		wantIP  string
		wantErr bool
	}{
		{
			name:   "linux format",
			output: "? (192.168.56.101) at 08:00:27:ee:a5:e7 [ether] on vboxnet0",
			wantIP: "192.168.56.101",
		},
		{
			name:   "macos format",
			output: "? (10.0.2.15) at 8:0:27:ab:cd:ef on en0 ifscope [ethernet]",
			wantIP: "10.0.2.15",
		},
		{
			name:   "hostname format",
			output: "my-vm (172.16.0.42) at 08:00:27:11:22:33 [ether] on eth0",
			wantIP: "172.16.0.42",
		},
		{
			name:    "no match",
			output:  "no arp entries here",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ip, err := parseIPFromARPOutput(tt.output)
			if tt.wantErr {
				if !errors.Is(err, errIPNotFoundInARP) {
					t.Fatalf("parseIPFromARPOutput() error = %v, want %v", err, errIPNotFoundInARP)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseIPFromARPOutput() error = %v", err)
			}
			if ip != tt.wantIP {
				t.Fatalf("ip = %q, want %q", ip, tt.wantIP)
			}
		})
	}
}

func TestIsGrepNoMatch(t *testing.T) {
	t.Parallel()

	if isGrepNoMatch(nil) {
		t.Fatal("expected nil error to not be a grep no-match")
	}

	err := exec.Command("sh", "-c", "grep example /dev/null").Run()
	if !isGrepNoMatch(err) {
		t.Fatalf("expected grep exit status 1 to be a no-match, got %v", err)
	}
}

func TestShellQuote(t *testing.T) {
	t.Parallel()

	if got := shellQuote(`08:00:27:ab'cd:ef`); got != `'08:00:27:ab'\''cd:ef'` {
		t.Fatalf("shellQuote() = %q", got)
	}
}
