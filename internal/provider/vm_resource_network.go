// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func networkAdaptersFromModel(models []networkAdapterModel) ([]vboxmanage.NetworkAdapter, diag.Diagnostics) {
	var diags diag.Diagnostics
	adapters := make([]vboxmanage.NetworkAdapter, len(models))

	for i, model := range models {
		adapter := vboxmanage.NetworkAdapter{
			Type:            model.Type.ValueString(),
			HostInterface:   model.HostInterface.ValueString(),
			PromiscuousMode: model.PromiscuousMode.ValueString(),
		}
		if err := vboxmanage.ValidateNetworkAdapter(adapter); err != nil {
			diags.AddError(
				"Invalid network adapter",
				fmt.Sprintf("network_adapter[%d]: %s", i, err.Error()),
			)
			continue
		}
		adapters[i] = adapter
	}

	return adapters, diags
}

func networkAdaptersToModel(adapters []vboxmanage.NetworkAdapter) []networkAdapterModel {
	models := make([]networkAdapterModel, len(adapters))
	for i, adapter := range adapters {
		models[i] = networkAdapterModel{
			Type:            types.StringValue(adapter.Type),
			PromiscuousMode: types.StringValue(vboxmanage.NormalizePromiscuousMode(adapter.PromiscuousMode)),
		}
		if adapter.HostInterface != "" {
			models[i].HostInterface = types.StringValue(adapter.HostInterface)
		} else {
			models[i].HostInterface = types.StringNull()
		}
	}
	return models
}

func networkAdaptersModelEqual(plan, state []networkAdapterModel) bool {
	if len(plan) != len(state) {
		return false
	}
	for i := range plan {
		if !plan[i].Type.Equal(state[i].Type) ||
			!plan[i].HostInterface.Equal(state[i].HostInterface) ||
			!plan[i].PromiscuousMode.Equal(state[i].PromiscuousMode) {
			return false
		}
	}
	return true
}
