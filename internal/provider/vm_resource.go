// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource = &vmResource{}
)

// NewVMResource is a helper function to simplify the provider implementation.
func NewVMResource() resource.Resource {
	return &vmResource{}
}

// vmResource is the resource implementation.
type vmResource struct {
	client vboxmanage.VirtualBox
}

type vmResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	OSType      types.String `tfsdk:"os_type"`
	CPUs        types.Int64  `tfsdk:"cpus"`
	Memory      types.Int64  `tfsdk:"memory"`
	LastUpdated types.String `tfsdk:"last_updated"`
}

// Metadata returns the resource type name.
func (r *vmResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm"
}

// Configure adds the provider configured client to the resource.
func (r *vmResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(vboxmanage.VirtualBox)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected vboxmanage.VirtualBox, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Schema defines the schema for the resource.
func (r *vmResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the virtual machine.",
			},
			"os_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "VirtualBox guest OS type identifier (for example, `Linux_64`).",
				Default:             stringdefault.StaticString("Linux_64"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cpus": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Number of virtual CPUs.",
				Default:             int64default.StaticInt64(1),
			},
			"memory": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Memory in megabytes.",
				Default:             int64default.StaticInt64(1024),
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *vmResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vmResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	vm, err := r.client.CreateVM(ctx, plan.Name.ValueString(), vboxmanage.CreateVMOptions{
		OSType: plan.OSType.ValueString(),
		CPUs:   int(plan.CPUs.ValueInt64()),
		Memory: int(plan.Memory.ValueInt64()),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating VM",
			"Could not create virtual machine, unexpected error:"+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(vm.UUID)
	plan.CPUs = types.Int64Value(int64(vm.CPUs))
	plan.Memory = types.Int64Value(int64(vm.Memory))
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *vmResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vmResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	vm, err := r.client.GetVM(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, vboxmanage.ErrVMNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading VM",
			"Could not read virtual machine, unexpected error:"+err.Error(),
		)
		return
	}

	state.Name = types.StringValue(vm.Name)
	state.ID = types.StringValue(vm.UUID)
	state.CPUs = types.Int64Value(int64(vm.CPUs))
	state.Memory = types.Int64Value(int64(vm.Memory))

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *vmResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// retrieve values from plan
	var plan vmResourceModel
	var state vmResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateOpts := vboxmanage.UpdateVMOptions{}

	if !plan.Name.Equal(state.Name) {
		name := plan.Name.ValueString()
		updateOpts.Name = &name
	}
	if !plan.CPUs.Equal(state.CPUs) {
		cpus := int(plan.CPUs.ValueInt64())
		updateOpts.CPUs = &cpus
	}
	if !plan.Memory.Equal(state.Memory) {
		memory := int(plan.Memory.ValueInt64())
		updateOpts.Memory = &memory
	}

	if updateOpts.HasChanges() {
		vm, err := r.client.UpdateVM(ctx, state.ID.ValueString(), updateOpts)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating VM",
				"Could not update virtual machine, unexpected error:"+err.Error(),
			)
			return
		}
		plan.Name = types.StringValue(vm.Name)
		plan.CPUs = types.Int64Value(int64(vm.CPUs))
		plan.Memory = types.Int64Value(int64(vm.Memory))
	}

	plan.ID = state.ID
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *vmResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteVM(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, vboxmanage.ErrVMNotFound) {
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting VM",
			"Could not delete virtual machine, unexpected error:"+err.Error(),
		)
		return
	}
}
