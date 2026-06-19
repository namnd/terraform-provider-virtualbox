// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import "testing"

func TestNormalizeOSTypeFromDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "Other Linux (64-bit)", want: "Linux_64"},
		{input: "Other Linux (ARM 64-bit)", want: "Linux_arm64"},
		{input: "Ubuntu (64-bit)", want: "Ubuntu_64"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeOSType(tt.input); got != tt.want {
				t.Fatalf("NormalizeOSType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeOSTypeFromID(t *testing.T) {
	t.Parallel()

	tests := []string{"Linux_64", "Linux_arm64", "Ubuntu_64"}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeOSType(input); got != input {
				t.Fatalf("NormalizeOSType(%q) = %q, want %q", input, got, input)
			}
		})
	}
}

func TestNormalizeOSTypeUnknown(t *testing.T) {
	t.Parallel()

	input := "Some Future OS (128-bit)"
	if got := NormalizeOSType(input); got != input {
		t.Fatalf("NormalizeOSType(%q) = %q, want passthrough", input, got)
	}
}
