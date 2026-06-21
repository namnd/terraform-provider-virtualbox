// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import "testing"

func TestValidateDesiredVMState(t *testing.T) {
	t.Parallel()

	valid := []string{
		DesiredVMStatePowerOff,
		DesiredVMStateRunning,
		DesiredVMStatePaused,
		DesiredVMStateSaved,
	}
	for _, state := range valid {
		if err := validateDesiredVMState(state); err != nil {
			t.Fatalf("validateDesiredVMState(%q) error = %v", state, err)
		}
	}

	if err := validateDesiredVMState("reboot"); err == nil {
		t.Fatal("expected reboot to be an invalid desired state")
	}
}

func TestValidateVMStartType(t *testing.T) {
	t.Parallel()

	for _, startType := range []string{VMStartTypeHeadless, VMStartTypeGUI} {
		if err := validateVMStartType(startType); err != nil {
			t.Fatalf("validateVMStartType(%q) error = %v", startType, err)
		}
	}

	if err := validateVMStartType("invalid"); err == nil {
		t.Fatal("expected invalid start type to fail validation")
	}
}

func TestStateMatchesDesired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		actual  string
		desired string
		want    bool
	}{
		{actual: "poweroff", desired: DesiredVMStatePowerOff, want: true},
		{actual: "aborted", desired: DesiredVMStatePowerOff, want: true},
		{actual: "running", desired: DesiredVMStateRunning, want: true},
		{actual: "paused", desired: DesiredVMStatePaused, want: true},
		{actual: "saved", desired: DesiredVMStateSaved, want: true},
		{actual: "poweroff", desired: DesiredVMStateRunning, want: false},
		{actual: "starting", desired: DesiredVMStateRunning, want: false},
	}

	for _, tt := range tests {
		if got := stateMatchesDesired(tt.actual, tt.desired); got != tt.want {
			t.Fatalf("stateMatchesDesired(%q, %q) = %v, want %v", tt.actual, tt.desired, got, tt.want)
		}
	}
}

func TestNormalizeVMStateForRead(t *testing.T) {
	t.Parallel()

	if got := normalizeVMStateForRead("aborted"); got != DesiredVMStatePowerOff {
		t.Fatalf("normalizeVMStateForRead(aborted) = %q, want %q", got, DesiredVMStatePowerOff)
	}
	if got := normalizeVMStateForRead("running"); got != "running" {
		t.Fatalf("normalizeVMStateForRead(running) = %q, want running", got)
	}
}
