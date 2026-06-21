// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func TestVMStateResourceConfigureAcceptsVirtualBox(t *testing.T) {
	t.Parallel()

	mock := &mockVirtualBox{}
	r := &vmStateResource{}
	resp := &resource.ConfigureResponse{}

	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: mock,
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure() diagnostics = %v", resp.Diagnostics.Errors())
	}
	if r.vbox != mock {
		t.Fatal("expected mock VirtualBox to be stored on resource")
	}
}

func TestVMStateResourceConfigureRejectsWrongType(t *testing.T) {
	t.Parallel()

	r := &vmStateResource{}
	resp := &resource.ConfigureResponse{}

	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: "not-a-virtualbox-client",
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected Configure() to fail for wrong provider data type")
	}
}

func TestShouldReboot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		desiredState string
		planTrigger  types.String
		stateTrigger types.String
		want         bool
	}{
		{
			name:         "running with new trigger",
			desiredState: vboxmanage.DesiredVMStateRunning,
			planTrigger:  types.StringValue("v2"),
			stateTrigger: types.StringValue("v1"),
			want:         true,
		},
		{
			name:         "running with unchanged trigger",
			desiredState: vboxmanage.DesiredVMStateRunning,
			planTrigger:  types.StringValue("v1"),
			stateTrigger: types.StringValue("v1"),
			want:         false,
		},
		{
			name:         "running with initial trigger",
			desiredState: vboxmanage.DesiredVMStateRunning,
			planTrigger:  types.StringValue("v1"),
			stateTrigger: types.StringNull(),
			want:         true,
		},
		{
			name:         "poweroff ignores trigger",
			desiredState: vboxmanage.DesiredVMStatePowerOff,
			planTrigger:  types.StringValue("v2"),
			stateTrigger: types.StringValue("v1"),
			want:         false,
		},
		{
			name:         "null trigger",
			desiredState: vboxmanage.DesiredVMStateRunning,
			planTrigger:  types.StringNull(),
			stateTrigger: types.StringValue("v1"),
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldReboot(tt.desiredState, tt.planTrigger, tt.stateTrigger); got != tt.want {
				t.Fatalf("shouldReboot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVMStateResourceApplyUsesVirtualBox(t *testing.T) {
	t.Parallel()

	var gotVMID string
	var gotState string
	var gotStartType string

	mock := &mockVirtualBox{
		setVMStateFn: func(_ context.Context, id string, desired string, opts vboxmanage.SetVMStateOptions) error {
			gotVMID = id
			gotState = desired
			gotStartType = opts.StartType
			return nil
		},
	}

	r := &vmStateResource{vbox: mock}
	if err := r.applyVMState(context.Background(), "vm-uuid", vboxmanage.DesiredVMStateRunning, vboxmanage.VMStartTypeGUI); err != nil {
		t.Fatalf("applyVMState() error = %v", err)
	}
	if gotVMID != "vm-uuid" {
		t.Fatalf("gotVMID = %q, want vm-uuid", gotVMID)
	}
	if gotState != vboxmanage.DesiredVMStateRunning {
		t.Fatalf("gotState = %q, want running", gotState)
	}
	if gotStartType != vboxmanage.VMStartTypeGUI {
		t.Fatalf("gotStartType = %q, want gui", gotStartType)
	}
}
