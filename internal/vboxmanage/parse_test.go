// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"strings"
	"testing"
)

func TestParseCreateVMOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		vmName  string
		stdout  string
		want    *VM
		wantErr string
	}{
		{
			name:   "parses uuid",
			vmName: "test-vm",
			stdout: "Virtual machine 'test-vm' is created and registered.\nUUID: abc-def-123\n",
			want: &VM{
				Name: "test-vm",
				UUID: "abc-def-123",
			},
		},
		{
			name:    "missing uuid",
			vmName:  "test-vm",
			stdout:  "Virtual machine 'test-vm' is created and registered.\n",
			wantErr: "UUID was not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseCreateVMOutput(tt.vmName, tt.stdout)
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
			if got.Name != tt.want.Name || got.UUID != tt.want.UUID {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseShowVMInfoOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		stdout  string
		want    *VM
		wantErr string
	}{
		{
			name: "parses name and uuid",
			stdout: `name="test-vm"
UUID="abc-def-123"
`,
			want: &VM{
				Name: "test-vm",
				UUID: "abc-def-123",
			},
		},
		{
			name:    "missing name",
			stdout:  `UUID="abc-def-123"`,
			wantErr: "name or UUID was not found",
		},
		{
			name:    "missing uuid",
			stdout:  `name="test-vm"`,
			wantErr: "name or UUID was not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseShowVMInfoOutput(tt.stdout)
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
			if got.Name != tt.want.Name || got.UUID != tt.want.UUID {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
