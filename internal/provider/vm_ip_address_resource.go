// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

const vmIPAddressCreateTimeout = 120 * time.Second

var _ resource.Resource = &vmIPAddressResource{}

// NewVMIPAddressResource is a helper function to simplify the provider implementation.
func NewVMIPAddressResource() resource.Resource {
	return &vmIPAddressResource{}
}

type vmIPAddressResource struct {
	client vboxmanage.VirtualBox
}

type vmIPAddressResourceModel struct {
	IP_Address  types.String   `tfsdk:"ip_address"`
	VMID        types.String   `tfsdk:"vm_id"`
	LastUpdated types.String   `tfsdk:"last_updated"`
	Timeouts    timeouts.Value `tfsdk:"timeouts"`
}

func (r *vmIPAddressResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_ip_address"
}

func (r *vmIPAddressResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *vmIPAddressResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"ip_address": schema.StringAttribute{
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
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
			}),
		},
	}
}

func (r *vmIPAddressResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vmIPAddressResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, vmIPAddressCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	vm, err := r.client.GetVM(ctx, plan.VMID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error validating VM",
			"Could not find virtual machine, unexpected error: "+err.Error(),
		)
		return
	}

	ip, err := r.client.GetVMIPAddress(ctx, plan.VMID.ValueString(), vboxmanage.GetVMIPAddressOptions{
		NetworkAdapters: vm.NetworkAdapters,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading IP address",
			"Could not get VM IP address, unexpected error: "+err.Error(),
		)
		return
	}

	plan.IP_Address = types.StringValue(*ip)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *vmIPAddressResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

func (r *vmIPAddressResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (r *vmIPAddressResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}
