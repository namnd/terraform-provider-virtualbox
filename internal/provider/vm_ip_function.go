// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

var (
	_ function.Function = &vmIPFunction{}

	vmIPResultAttributeTypes = map[string]attr.Type{
		"ip_address":      types.StringType,
		"mac_address":     types.StringType,
		"network_adapter": types.Int64Type,
		"timeout":         types.StringType,
	}
)

// NewVMIPFunction is a helper function to simplify the provider implementation.
func NewVMIPFunction(provider *VirtualboxProvider) function.Function {
	return &vmIPFunction{provider: provider}
}

type vmIPFunction struct {
	provider *VirtualboxProvider
}

func (f *vmIPFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "vm_ip"
}

func (f *vmIPFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Resolve a virtual machine IP address",
		MarkdownDescription: "Starts the VM headless when needed, resolves the adapter MAC address, and looks up its IP via ARP. The VM is powered off after the IP is resolved successfully and only when this call started the VM.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "vm_id",
				MarkdownDescription: "UUID or name of the virtual machine.",
			},
			function.Int64Parameter{
				Name:                "network_adapter",
				AllowNullValue:      true,
				MarkdownDescription: "Zero-based index of the network adapter to resolve. Defaults to the first adapter when null.",
			},
			function.StringParameter{
				Name:                "timeout",
				AllowNullValue:      true,
				MarkdownDescription: "Maximum time to wait for the VM IP address to appear in the ARP table. Must be a duration string such as `60s` or `2m`. Defaults to `60s` when null.",
			},
		},
		Return: function.ObjectReturn{
			AttributeTypes: vmIPResultAttributeTypes,
		},
	}
}

func (f *vmIPFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	vbox, err := f.provider.vboxClient(ctx)
	if err != nil {
		resp.Error = function.NewFuncError(err.Error())
		return
	}

	var vmID types.String
	var networkAdapter types.Int64
	var timeout types.String

	resp.Error = function.ConcatFuncErrors(
		resp.Error,
		req.Arguments.Get(ctx, &vmID, &networkAdapter, &timeout),
	)
	if resp.Error != nil {
		return
	}

	adapterIndex, timeoutValue, parsedTimeout, err := parseVMIPOptions(networkAdapter, timeout)
	if err != nil {
		resp.Error = function.NewFuncError(err.Error())
		return
	}

	vmIP, err := vbox.GetVMIP(ctx, vmID.ValueString(), vboxmanage.GetVMIPOptions{
		NetworkAdapter: adapterIndex,
		Timeout:        parsedTimeout,
	})
	if err != nil {
		resp.Error = function.NewFuncError(
			fmt.Sprintf("Could not resolve IP address for VM %q: %s", vmID.ValueString(), err),
		)
		return
	}

	result, diags := types.ObjectValue(vmIPResultAttributeTypes, map[string]attr.Value{
		"ip_address":      types.StringValue(vmIP.IPAddress),
		"mac_address":     types.StringValue(vmIP.MACAddress),
		"network_adapter": types.Int64Value(int64(adapterIndex)),
		"timeout":         types.StringValue(timeoutValue),
	})
	if diags.HasError() {
		resp.Error = function.FuncErrorFromDiags(ctx, diags)
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, result))
}

func parseVMIPOptions(networkAdapter types.Int64, timeout types.String) (adapterIndex int, timeoutValue string, parsedTimeout time.Duration, err error) {
	const defaultTimeoutValue = "60s"

	adapterIndex = 0
	if !networkAdapter.IsNull() && !networkAdapter.IsUnknown() {
		adapterIndex = int(networkAdapter.ValueInt64())
	}

	timeoutValue = defaultTimeoutValue
	parsedTimeout = vboxmanage.DefaultVMIPLookupTimeout()
	if !timeout.IsNull() && !timeout.IsUnknown() {
		timeoutValue = timeout.ValueString()
		parsedTimeout, err = time.ParseDuration(timeoutValue)
		if err != nil {
			return 0, "", 0, fmt.Errorf("could not parse timeout %q: %s", timeoutValue, err)
		}
		if parsedTimeout <= 0 {
			return 0, "", 0, fmt.Errorf("timeout must be greater than zero")
		}
	}

	return adapterIndex, timeoutValue, parsedTimeout, nil
}
