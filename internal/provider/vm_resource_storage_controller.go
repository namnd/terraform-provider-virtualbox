// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namnd/terraform-provider-virtualbox/internal/vboxmanage"
)

func storageControllersFromModel(models []storageControllerModel) ([]vboxmanage.StorageController, diag.Diagnostics) {
	var diags diag.Diagnostics
	controllers := make([]vboxmanage.StorageController, len(models))

	for i, model := range models {
		controller := vboxmanage.StorageController{
			Name:        model.Name.ValueString(),
			Type:        model.Type.ValueString(),
			Controller:  model.Controller.ValueString(),
			Bootable:    storageBootableFromModel(model.Bootable),
			HostIOCache: storageHostIOCacheFromModel(model.HostIOCache),
		}
		if !model.PortCount.IsNull() && !model.PortCount.IsUnknown() {
			controller.PortCount = int(model.PortCount.ValueInt64())
		}

		if err := vboxmanage.ValidateStorageController(controller); err != nil {
			diags.AddError(
				"Invalid storage controller",
				fmt.Sprintf("storage_controller[%d]: %s", i, err.Error()),
			)
			continue
		}
		controllers[i] = controller
	}

	return controllers, diags
}

func storageControllersToModel(controllers []vboxmanage.StorageController) []storageControllerModel {
	models := make([]storageControllerModel, len(controllers))
	for i, controller := range controllers {
		models[i] = storageControllerModel{
			Name:        types.StringValue(controller.Name),
			Type:        types.StringValue(controller.Type),
			Controller:  types.StringValue(vboxmanage.NormalizeStorageControllerChip(controller.Controller)),
			Bootable:    storageBootableToModel(controller.Bootable),
			HostIOCache: storageHostIOCacheToModel(controller.HostIOCache),
		}
		if controller.PortCount > 0 {
			models[i].PortCount = types.Int64Value(int64(controller.PortCount))
		} else {
			models[i].PortCount = types.Int64Null()
		}
	}
	return models
}

func storageControllersModelEqual(plan, state []storageControllerModel) bool {
	if len(plan) != len(state) {
		return false
	}
	for i := range plan {
		if !plan[i].Name.Equal(state[i].Name) ||
			!plan[i].Type.Equal(state[i].Type) ||
			!plan[i].Controller.Equal(state[i].Controller) ||
			!plan[i].Bootable.Equal(state[i].Bootable) ||
			!plan[i].HostIOCache.Equal(state[i].HostIOCache) ||
			!plan[i].PortCount.Equal(state[i].PortCount) {
			return false
		}
	}
	return true
}

func storageBootableFromModel(value types.Bool) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	if value.ValueBool() {
		return vboxmanage.StorageBootableOn
	}
	return vboxmanage.StorageBootableOff
}

func storageBootableToModel(value string) types.Bool {
	return types.BoolValue(vboxmanage.NormalizeStorageBootable(value) == vboxmanage.StorageBootableOn)
}

func storageHostIOCacheFromModel(value types.Bool) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	if value.ValueBool() {
		return vboxmanage.StorageHostIOCacheOn
	}
	return vboxmanage.StorageHostIOCacheOff
}

func storageHostIOCacheToModel(value string) types.Bool {
	return types.BoolValue(vboxmanage.NormalizeStorageHostIOCache(value) == vboxmanage.StorageHostIOCacheOn)
}
