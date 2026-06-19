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

var _ resource.Resource = &diskResource{}
var _ resource.ResourceWithImportState = &diskResource{}

// NewDiskResource is a helper function to simplify the provider implementation.
func NewDiskResource() resource.Resource {
	return &diskResource{}
}

type diskResource struct {
	vbox vboxmanage.VirtualBox
}

type diskResourceModel struct {
	ID          types.String `tfsdk:"id"`
	FilePath    types.String `tfsdk:"file_path"`
	Size        types.Int64  `tfsdk:"size"`
	Format      types.String `tfsdk:"format"`
	Variant     types.String `tfsdk:"variant"`
	LastUpdated types.String `tfsdk:"last_updated"`
}

func (r *diskResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_disk"
}

func (r *diskResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *diskResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UUID of the disk medium.",
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
			},
			"file_path": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Absolute path for the disk file.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"size": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Disk size in megabytes.",
			},
			"format": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Disk format. Must be one of: `VDI`, `VMDK`, `VHD`.",
				Default:             stringdefault.StaticString(vboxmanage.DiskFormatVDI),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"variant": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Disk variant. Must be one of: `Standard`, `Fixed`.",
				Default:             stringdefault.StaticString(vboxmanage.DiskVariantStandard),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *diskResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan diskResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	disk, err := r.vbox.CreateDisk(ctx, vboxmanage.CreateDiskOptions{
		FilePath: plan.FilePath.ValueString(),
		Size:     int(plan.Size.ValueInt64()),
		Format:   plan.Format.ValueString(),
		Variant:  plan.Variant.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating disk",
			"Could not create disk medium, unexpected error: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(disk.UUID)
	plan.FilePath = types.StringValue(disk.FilePath)
	plan.Size = types.Int64Value(int64(disk.Size))
	plan.Format = types.StringValue(disk.Format)
	plan.Variant = types.StringValue(disk.Variant)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *diskResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state diskResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	disk, err := r.vbox.GetDisk(ctx, state.FilePath.ValueString())
	if err != nil {
		if errors.Is(err, vboxmanage.ErrMediumNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading disk",
			"Could not read disk medium, unexpected error: "+err.Error(),
		)
		return
	}

	if !disk.Accessible {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(disk.UUID)
	state.FilePath = types.StringValue(disk.FilePath)
	state.Size = types.Int64Value(int64(disk.Size))
	state.Format = types.StringValue(disk.Format)
	state.Variant = types.StringValue(disk.Variant)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *diskResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan diskResourceModel
	var state diskResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.Size.Equal(state.Size) {
		size := int(plan.Size.ValueInt64())
		disk, err := r.vbox.UpdateDisk(ctx, state.FilePath.ValueString(), vboxmanage.UpdateDiskOptions{
			Size: &size,
		})
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating disk",
				"Could not update disk medium, unexpected error: "+err.Error(),
			)
			return
		}
		plan.Size = types.Int64Value(int64(disk.Size))
		plan.Format = types.StringValue(disk.Format)
		plan.Variant = types.StringValue(disk.Variant)
	}

	plan.ID = state.ID
	plan.FilePath = state.FilePath
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *diskResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state diskResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.vbox.DeleteDisk(ctx, state.FilePath.ValueString())
	if err != nil {
		if errors.Is(err, vboxmanage.ErrMediumNotFound) {
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting disk",
			"Could not delete disk medium, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *diskResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	disk, err := r.vbox.GetDisk(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error importing disk",
			fmt.Sprintf("Could not read disk medium %q: %s", req.ID, err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), disk.UUID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("file_path"), disk.FilePath)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("size"), int64(disk.Size))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("format"), disk.Format)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("variant"), disk.Variant)...)
}
