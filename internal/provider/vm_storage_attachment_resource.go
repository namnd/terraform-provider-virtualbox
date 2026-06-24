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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

var _ resource.Resource = &vmStorageAttachmentResource{}

// NewVMStorageAttachmentResource is a helper function to simplify the provider implementation.
func NewVMStorageAttachmentResource() resource.Resource {
	return &vmStorageAttachmentResource{}
}

type vmStorageAttachmentResource struct {
	client vboxmanage.VirtualBox
}

type vmStorageAttachmentResourceModel struct {
	ID             types.String `tfsdk:"id"`
	VMID           types.String `tfsdk:"vm_id"`
	ControllerName types.String `tfsdk:"controller_name"`
	Port           types.Int64  `tfsdk:"port"`
	Device         types.Int64  `tfsdk:"device"`
	Type           types.String `tfsdk:"type"`
	Medium         types.String `tfsdk:"medium"`
	MediumType     types.String `tfsdk:"medium_type"`
	LastUpdated    types.String `tfsdk:"last_updated"`
}

func (r *vmStorageAttachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_storage_attachment"
}

func (r *vmStorageAttachmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *vmStorageAttachmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Storage attachment identifier in the form `<vm_id>/<controller_name>/<port>/<device>`.",
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
			},
			"vm_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the virtual machine.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"controller_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the storage controller on the VM. Must match a `storage_controller.name` on the target VM.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"port": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Port number on the storage controller.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"device": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Device number on the storage controller port.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Drive type. Must be one of: `hdd`, `dvddrive`, `fdd`.",
				Default:             stringdefault.StaticString(vboxmanage.StorageAttachmentTypeHDD),
			},
			"medium": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Medium to attach. May be a disk UUID, absolute file path, `none`, or `emptydrive`.",
			},
			"medium_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Medium write mode. Must be one of: `normal`, `writethrough`, `immutable`, `shareable`, `readonly`, `multiattach`.",
				Default:             stringdefault.StaticString(vboxmanage.StorageMediumTypeNormal),
			},
		},
	}
}

func (r *vmStorageAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vmStorageAttachmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attachment, err := r.client.CreateStorageAttachment(ctx, plan.VMID.ValueString(), vboxmanage.CreateStorageAttachmentOptions{
		ControllerName: plan.ControllerName.ValueString(),
		Port:           int(plan.Port.ValueInt64()),
		Device:         int(plan.Device.ValueInt64()),
		Type:           plan.Type.ValueString(),
		Medium:         plan.Medium.ValueString(),
		MediumType:     plan.MediumType.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating VM storage attachment",
			"Could not attach storage medium, unexpected error: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(vboxmanage.FormatStorageAttachmentID(
		plan.VMID.ValueString(),
		plan.ControllerName.ValueString(),
		int(plan.Port.ValueInt64()),
		int(plan.Device.ValueInt64()),
	))
	plan.Type = types.StringValue(attachment.Type)
	plan.Medium = types.StringValue(attachment.Medium)
	plan.MediumType = types.StringValue(vboxmanage.NormalizeStorageMediumType(attachment.MediumType))
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *vmStorageAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vmStorageAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attachment, err := r.client.GetStorageAttachment(
		ctx,
		state.VMID.ValueString(),
		state.ControllerName.ValueString(),
		int(state.Port.ValueInt64()),
		int(state.Device.ValueInt64()),
	)
	if err != nil {
		if errors.Is(err, vboxmanage.ErrStorageAttachmentNotFound) || errors.Is(err, vboxmanage.ErrVMNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading VM storage attachment",
			"Could not read storage attachment, unexpected error: "+err.Error(),
		)
		return
	}

	state.ID = types.StringValue(vboxmanage.FormatStorageAttachmentID(
		state.VMID.ValueString(),
		state.ControllerName.ValueString(),
		int(state.Port.ValueInt64()),
		int(state.Device.ValueInt64()),
	))
	state.Type = types.StringValue(attachment.Type)
	state.Medium = types.StringValue(attachment.Medium)
	state.MediumType = types.StringValue(vboxmanage.NormalizeStorageMediumType(attachment.MediumType))

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *vmStorageAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan vmStorageAttachmentResourceModel
	var state vmStorageAttachmentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateOpts := vboxmanage.UpdateStorageAttachmentOptions{}

	if !plan.Type.Equal(state.Type) {
		attachmentType := plan.Type.ValueString()
		updateOpts.Type = &attachmentType
	}
	if !plan.Medium.Equal(state.Medium) {
		medium := plan.Medium.ValueString()
		updateOpts.Medium = &medium
	}
	if !plan.MediumType.Equal(state.MediumType) {
		mediumType := plan.MediumType.ValueString()
		updateOpts.MediumType = &mediumType
	}

	attachment, err := r.client.UpdateStorageAttachment(
		ctx,
		state.VMID.ValueString(),
		state.ControllerName.ValueString(),
		int(state.Port.ValueInt64()),
		int(state.Device.ValueInt64()),
		updateOpts,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating VM storage attachment",
			"Could not update storage attachment, unexpected error: "+err.Error(),
		)
		return
	}

	plan.ID = state.ID
	plan.VMID = state.VMID
	plan.ControllerName = state.ControllerName
	plan.Port = state.Port
	plan.Device = state.Device
	plan.Type = types.StringValue(attachment.Type)
	plan.Medium = types.StringValue(attachment.Medium)
	plan.MediumType = types.StringValue(vboxmanage.NormalizeStorageMediumType(attachment.MediumType))
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *vmStorageAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmStorageAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteStorageAttachment(
		ctx,
		state.VMID.ValueString(),
		state.ControllerName.ValueString(),
		int(state.Port.ValueInt64()),
		int(state.Device.ValueInt64()),
	)
	if err != nil {
		if errors.Is(err, vboxmanage.ErrVMNotFound) || errors.Is(err, vboxmanage.ErrStorageAttachmentNotFound) {
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting VM storage attachment",
			"Could not delete storage attachment, unexpected error: "+err.Error(),
		)
	}
}
