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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

var _ resource.Resource = &vmStorageResource{}

// NewVMStorageResource is a helper function to simplify the provider implementation.
func NewVMStorageResource() resource.Resource {
	return &vmStorageResource{}
}

type vmStorageResource struct {
	vbox vboxmanage.VirtualBox
}

type storageAttachmentModel struct {
	Port   types.Int64  `tfsdk:"port"`
	Device types.Int64  `tfsdk:"device"`
	Type   types.String `tfsdk:"type"`
	Medium types.String `tfsdk:"medium"`
}

type vmStorageResourceModel struct {
	ID                types.String           `tfsdk:"id"`
	VMID              types.String           `tfsdk:"vm_id"`
	Name              types.String           `tfsdk:"name"`
	Type              types.String           `tfsdk:"type"`
	Controller        types.String           `tfsdk:"controller"`
	PortCount         types.Int64            `tfsdk:"port_count"`
	HostIOCache       types.Bool             `tfsdk:"host_io_cache"`
	Bootable          types.Bool             `tfsdk:"bootable"`
	StorageAttachment storageAttachmentModel `tfsdk:"storage_attachment"`
	LastUpdated       types.String           `tfsdk:"last_updated"`
}

func (r *vmStorageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_storage"
}

func (r *vmStorageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *vmStorageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a VirtualBox storage controller with a single child storage attachment. The target VM is powered off automatically before storage changes are applied.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite identifier: `{vm_id}/{name}/{port}/{device}`.",
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
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the storage controller.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Storage bus type. Must be one of: `ide`, `sata`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"controller": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Storage controller chipset (for example, `PIIX4`, `IntelAhci`). Defaults based on `type`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"port_count": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Number of ports on the storage controller. `0` uses the VirtualBox default.",
				Default:             int64default.StaticInt64(0),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"host_io_cache": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Enable host I/O cache for the storage controller.",
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"bootable": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Mark the storage controller as bootable.",
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"storage_attachment": schema.SingleNestedBlock{
				MarkdownDescription: "Storage medium attached to this controller.",
				Attributes: map[string]schema.Attribute{
					"port": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: "Controller port for the storage attachment.",
						PlanModifiers: []planmodifier.Int64{
							int64planmodifier.RequiresReplace(),
						},
					},
					"device": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: "Controller device for the storage attachment.",
						PlanModifiers: []planmodifier.Int64{
							int64planmodifier.RequiresReplace(),
						},
					},
					"type": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Attachment type. Must be one of: `dvddrive`, `hdd`, `fdd`.",
					},
					"medium": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Path, UUID, or special medium value to attach.",
					},
				},
			},
		},
	}
}

func (r *vmStorageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vmStorageResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vmID := plan.VMID.ValueString()
	if _, err := r.vbox.GetVMRetry(ctx, vmID); err != nil {
		resp.Diagnostics.AddError(
			"Error validating VM",
			"Could not find virtual machine, unexpected error: "+err.Error(),
		)
		return
	}

	ctl, diags := vmStorageFromModel(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.vbox.CreateVMStorage(ctx, vmID, ctl); err != nil {
		resp.Diagnostics.AddError(
			"Error creating VM storage",
			"Could not create VM storage, unexpected error: "+err.Error(),
		)
		return
	}

	attach := ctl.Attachment
	plan.ID = types.StringValue(vmStorageResourceID(vmID, ctl.Name, int64(attach.Port), int64(attach.Device)))
	plan.Controller = types.StringValue(ctl.Controller)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *vmStorageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vmStorageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vmID := state.VMID.ValueString()
	attach := state.StorageAttachment
	storage, err := r.vbox.GetVMStorageRetry(ctx, vmID, state.Name.ValueString(), int(attach.Port.ValueInt64()), int(attach.Device.ValueInt64()))
	if err != nil {
		if errors.Is(err, vboxmanage.ErrVMNotFound) || errors.Is(err, vboxmanage.ErrVMStorageNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading VM storage",
			"Could not read VM storage, unexpected error: "+err.Error(),
		)
		return
	}

	state = vmStorageModelAfterRead(state, vmID, storage)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *vmStorageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan vmStorageResourceModel
	var state vmStorageResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if vmStorageAttachmentChanged(plan, state) {
		ctl, diags := vmStorageFromModel(plan)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		if err := r.vbox.AttachStorage(ctx, state.VMID.ValueString(), state.Name.ValueString(), ctl.Attachment); err != nil {
			resp.Diagnostics.AddError(
				"Error updating storage attachment",
				"Could not update storage attachment, unexpected error: "+err.Error(),
			)
			return
		}

		plan.StorageAttachment = storageAttachmentModel{
			Port:   types.Int64Value(int64(ctl.Attachment.Port)),
			Device: types.Int64Value(int64(ctl.Attachment.Device)),
			Type:   types.StringValue(ctl.Attachment.Type),
			Medium: types.StringValue(ctl.Attachment.Medium),
		}
	}

	plan.ID = state.ID
	plan.VMID = state.VMID
	plan.Name = state.Name
	plan.Type = state.Type
	plan.Controller = state.Controller
	plan.PortCount = state.PortCount
	plan.HostIOCache = state.HostIOCache
	plan.Bootable = state.Bootable
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *vmStorageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmStorageResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctl, diags := vmStorageDeleteFromState(state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.vbox.DeleteVMStorage(ctx, state.VMID.ValueString(), ctl); err != nil {
		resp.Diagnostics.AddError(
			"Error deleting VM storage",
			"Could not delete VM storage, unexpected error: "+err.Error(),
		)
		return
	}
}
