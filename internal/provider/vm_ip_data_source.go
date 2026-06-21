// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

var _ datasource.DataSource = &vmIPDataSource{}

// NewVMIPDataSource is a helper function to simplify the provider implementation.
func NewVMIPDataSource() datasource.DataSource {
	return &vmIPDataSource{}
}

type vmIPDataSource struct {
	vbox vboxmanage.VirtualBox
}

type vmIPDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	IPAddress      types.String `tfsdk:"ip_address"`
	MACAddress     types.String `tfsdk:"mac_address"`
	NetworkAdapter types.Int64  `tfsdk:"network_adapter"`
	Timeout        types.String `tfsdk:"timeout"`
}

func (d *vmIPDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_ip"
}

func (d *vmIPDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.vbox = vbox
}

func (d *vmIPDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the virtual machine.",
			},
			"ip_address": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "IP address of the virtual machine, resolved via ARP lookup after starting the VM headless.",
			},
			"mac_address": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "MAC address of the network adapter used for IP resolution.",
			},
			"network_adapter": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Zero-based index of the network adapter to resolve. Defaults to the first adapter.",
			},
			"timeout": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Maximum time to wait for the VM IP address to appear in the ARP table. Must be a duration string such as `60s` or `2m`. Defaults to `60s`.",
			},
		},
	}
}

func (d *vmIPDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config vmIPDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	adapterIndex := int64(0)
	if !config.NetworkAdapter.IsNull() && !config.NetworkAdapter.IsUnknown() {
		adapterIndex = config.NetworkAdapter.ValueInt64()
	}

	const defaultTimeoutValue = "60s"

	timeout := vboxmanage.DefaultVMIPLookupTimeout()
	timeoutValue := defaultTimeoutValue
	if !config.Timeout.IsNull() && !config.Timeout.IsUnknown() {
		timeoutValue = config.Timeout.ValueString()
		parsed, err := time.ParseDuration(timeoutValue)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid timeout value",
				fmt.Sprintf("Could not parse timeout %q: %s", timeoutValue, err),
			)
			return
		}
		if parsed <= 0 {
			resp.Diagnostics.AddError(
				"Invalid timeout value",
				"Timeout must be greater than zero.",
			)
			return
		}
		timeout = parsed
	}

	vmIP, err := d.vbox.GetVMIP(ctx, config.ID.ValueString(), vboxmanage.GetVMIPOptions{
		NetworkAdapter: int(adapterIndex),
		Timeout:        timeout,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read virtual machine IP address",
			fmt.Sprintf("Could not resolve IP address for VM %q: %s", config.ID.ValueString(), err),
		)
		return
	}

	config.IPAddress = types.StringValue(vmIP.IPAddress)
	config.MACAddress = types.StringValue(vmIP.MACAddress)
	config.NetworkAdapter = types.Int64Value(adapterIndex)
	config.Timeout = types.StringValue(timeoutValue)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
