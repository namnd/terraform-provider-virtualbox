// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

var (
	_ resource.Resource                = &vmStateResource{}
	_ resource.ResourceWithImportState = &vmStateResource{}
)

// NewVMStateResource is a helper function to simplify the provider implementation.
func NewVMStateResource() resource.Resource {
	return &vmStateResource{}
}

type vmStateResource struct {
	vbox vboxmanage.VirtualBox
}

type vmStateResourceModel struct {
	ID            types.String `tfsdk:"id"`
	VMID          types.String `tfsdk:"vm_id"`
	State         types.String `tfsdk:"state"`
	StartType     types.String `tfsdk:"start_type"`
	RebootTrigger types.String `tfsdk:"reboot_trigger"`
	LastUpdated   types.String `tfsdk:"last_updated"`
}

func (r *vmStateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_state"
}

func (r *vmStateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	vbox, ok := req.ProviderData.(vboxmanage.VirtualBox)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected vboxmanage.VirtualBox, got %T", req.ProviderData),
		)
		return
	}

	r.vbox = vbox
}

func (r *vmStateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the power state of a VirtualBox virtual machine.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UUID of the virtual machine.",
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
			},
			"vm_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID or name of the virtual machine.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"state": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Desired virtual machine state. Must be one of: `poweroff`, `running`, `paused`, `saved`.",
			},
			"start_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "How to start the VM when transitioning to `running`, `paused`, or `saved`. Must be one of: `headless`, `gui`. Defaults to `headless`.",
				Default:             stringdefault.StaticString(vboxmanage.VMStartTypeHeadless),
			},
			"reboot_trigger": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Change this value to hard-reset the VM while keeping `state = \"running\"`.",
			},
		},
	}
}

func (r *vmStateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vmStateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyVMState(ctx, plan.VMID.ValueString(), plan.State.ValueString(), plan.StartType.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error setting virtual machine state", err.Error())
		return
	}

	if shouldReboot(plan.State.ValueString(), plan.RebootTrigger, types.StringNull()) {
		if err := r.vbox.RebootVM(ctx, plan.VMID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error rebooting virtual machine", err.Error())
			return
		}
	}

	plan.ID = plan.VMID
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *vmStateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vmStateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vmState, err := r.vbox.GetVMState(ctx, state.VMID.ValueString())
	if err != nil {
		if errors.Is(err, vboxmanage.ErrVMNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading virtual machine state",
			"Could not read virtual machine state, unexpected error: "+err.Error(),
		)
		return
	}

	state.ID = state.VMID
	state.State = types.StringValue(vmState)
	state.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *vmStateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan vmStateResourceModel
	var state vmStateResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.State.Equal(state.State) || !plan.StartType.Equal(state.StartType) {
		if err := r.applyVMState(ctx, plan.VMID.ValueString(), plan.State.ValueString(), plan.StartType.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error setting virtual machine state", err.Error())
			return
		}
	}

	if shouldReboot(plan.State.ValueString(), plan.RebootTrigger, state.RebootTrigger) {
		if err := r.vbox.RebootVM(ctx, plan.VMID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error rebooting virtual machine", err.Error())
			return
		}
	}

	plan.ID = plan.VMID
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *vmStateResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *vmStateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	vmState, err := r.vbox.GetVMState(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error importing virtual machine state",
			fmt.Sprintf("Could not read virtual machine state for %q: %s", req.ID, err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vm_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("state"), vmState)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("start_type"), vboxmanage.VMStartTypeHeadless)...)
}

func (r *vmStateResource) applyVMState(ctx context.Context, vmID, desiredState, startType string) error {
	return r.vbox.SetVMState(ctx, vmID, desiredState, vboxmanage.SetVMStateOptions{
		StartType: startType,
	})
}

func shouldReboot(desiredState string, planTrigger, stateTrigger types.String) bool {
	if desiredState != vboxmanage.DesiredVMStateRunning {
		return false
	}
	if planTrigger.IsNull() || planTrigger.IsUnknown() {
		return false
	}
	if stateTrigger.IsNull() || stateTrigger.IsUnknown() {
		return true
	}
	return !planTrigger.Equal(stateTrigger)
}
