// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type vmTestAttributeValues struct {
	Strings         map[string]types.String
	Int64s          map[string]types.Int64
	NetworkAdapters *[]networkAdapterModel
}

func vmTestSchema(t *testing.T) schema.Schema {
	t.Helper()

	r := NewVMResource()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	return resp.Schema
}

func vmTestPlan(t *testing.T, schema schema.Schema, attrs vmTestAttributeValues) tfsdk.Plan {
	t.Helper()

	return tfsdk.Plan{
		Schema: schema,
		Raw:    vmTestObjectValue(t, schema, attrs),
	}
}

func vmTestState(t *testing.T, schema schema.Schema, attrs vmTestAttributeValues) tfsdk.State {
	t.Helper()

	return tfsdk.State{
		Schema: schema,
		Raw:    vmTestObjectValue(t, schema, attrs),
	}
}

func vmTestObjectValue(t *testing.T, s schema.Schema, attrs vmTestAttributeValues) tftypes.Value {
	t.Helper()

	ctx := context.Background()
	objectType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("expected tftypes.Object, got %T", s.Type().TerraformType(ctx))
	}
	tfAttrs := make(map[string]tftypes.Value, len(objectType.AttributeTypes))

	for name, attrType := range objectType.AttributeTypes {
		if attrs.NetworkAdapters != nil && name == "network_adapter" {
			tfAttrs[name] = vmTestNetworkAdapterListValue(t, attrType, *attrs.NetworkAdapters)
			continue
		}

		if value, ok := attrs.Strings[name]; ok {
			tfValue, err := value.ToTerraformValue(ctx)
			if err != nil {
				t.Fatalf("failed to convert string %q to terraform value: %v", name, err)
			}
			tfAttrs[name] = tfValue
			continue
		}

		if value, ok := attrs.Int64s[name]; ok {
			tfValue, err := value.ToTerraformValue(ctx)
			if err != nil {
				t.Fatalf("failed to convert int64 %q to terraform value: %v", name, err)
			}
			tfAttrs[name] = tfValue
			continue
		}

		tfAttrs[name] = tftypes.NewValue(attrType, nil)
	}

	return tftypes.NewValue(objectType, tfAttrs)
}

func vmTestNetworkAdapterListValue(t *testing.T, listType tftypes.Type, adapters []networkAdapterModel) tftypes.Value {
	t.Helper()

	ctx := context.Background()

	list, ok := listType.(tftypes.List)
	if !ok {
		t.Fatalf("expected tftypes.List, got %T", listType)
	}

	elements := make([]tftypes.Value, len(adapters))
	for i, adapter := range adapters {
		typeVal, err := adapter.Type.ToTerraformValue(ctx)
		if err != nil {
			t.Fatalf("failed to convert network adapter type to terraform value: %v", err)
		}
		hostIfaceVal, err := adapter.HostInterface.ToTerraformValue(ctx)
		if err != nil {
			t.Fatalf("failed to convert network adapter host_interface to terraform value: %v", err)
		}
		promiscVal, err := adapter.PromiscuousMode.ToTerraformValue(ctx)
		if err != nil {
			t.Fatalf("failed to convert network adapter promiscuous_mode to terraform value: %v", err)
		}
		macVal, err := adapter.MACAddress.ToTerraformValue(ctx)
		if err != nil {
			t.Fatalf("failed to convert network adapter mac_address to terraform value: %v", err)
		}

		elements[i] = tftypes.NewValue(list.ElementType, map[string]tftypes.Value{
			"type":             typeVal,
			"host_interface":   hostIfaceVal,
			"promiscuous_mode": promiscVal,
			"mac_address":      macVal,
		})
	}

	return tftypes.NewValue(listType, elements)
}

func vmGetStateModel(t *testing.T, ctx context.Context, state tfsdk.State) vmResourceModel {
	t.Helper()

	var model vmResourceModel
	diags := state.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("state.Get diagnostics: %v", diags)
	}

	return model
}

type diskTestAttributeValues struct {
	Strings map[string]types.String
	Int64s  map[string]types.Int64
}

func diskTestSchema(t *testing.T) schema.Schema {
	t.Helper()

	r := NewDiskResource()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	return resp.Schema
}

func diskTestPlan(t *testing.T, schema schema.Schema, attrs diskTestAttributeValues) tfsdk.Plan {
	t.Helper()

	return tfsdk.Plan{
		Schema: schema,
		Raw:    diskTestObjectValue(t, schema, attrs),
	}
}

func diskTestState(t *testing.T, schema schema.Schema, attrs diskTestAttributeValues) tfsdk.State {
	t.Helper()

	return tfsdk.State{
		Schema: schema,
		Raw:    diskTestObjectValue(t, schema, attrs),
	}
}

func diskTestObjectValue(t *testing.T, s schema.Schema, attrs diskTestAttributeValues) tftypes.Value {
	t.Helper()

	ctx := context.Background()
	objectType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("expected tftypes.Object, got %T", s.Type().TerraformType(ctx))
	}
	tfAttrs := make(map[string]tftypes.Value, len(objectType.AttributeTypes))

	for name, attrType := range objectType.AttributeTypes {
		if value, ok := attrs.Strings[name]; ok {
			tfValue, err := value.ToTerraformValue(ctx)
			if err != nil {
				t.Fatalf("failed to convert string %q to terraform value: %v", name, err)
			}
			tfAttrs[name] = tfValue
			continue
		}

		if value, ok := attrs.Int64s[name]; ok {
			tfValue, err := value.ToTerraformValue(ctx)
			if err != nil {
				t.Fatalf("failed to convert int64 %q to terraform value: %v", name, err)
			}
			tfAttrs[name] = tfValue
			continue
		}

		tfAttrs[name] = tftypes.NewValue(attrType, nil)
	}

	return tftypes.NewValue(objectType, tfAttrs)
}

type vmStorageAttachmentTestAttributeValues struct {
	Strings map[string]types.String
	Int64s  map[string]types.Int64
}

func vmStorageAttachmentTestSchema(t *testing.T) schema.Schema {
	t.Helper()

	r := NewVMStorageAttachmentResource()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	return resp.Schema
}

func vmStorageAttachmentTestPlan(t *testing.T, schema schema.Schema, attrs vmStorageAttachmentTestAttributeValues) tfsdk.Plan {
	t.Helper()

	return tfsdk.Plan{
		Schema: schema,
		Raw:    vmStorageAttachmentTestObjectValue(t, schema, attrs),
	}
}

func vmStorageAttachmentTestState(t *testing.T, schema schema.Schema, attrs vmStorageAttachmentTestAttributeValues) tfsdk.State {
	t.Helper()

	return tfsdk.State{
		Schema: schema,
		Raw:    vmStorageAttachmentTestObjectValue(t, schema, attrs),
	}
}

func vmStorageAttachmentTestObjectValue(t *testing.T, s schema.Schema, attrs vmStorageAttachmentTestAttributeValues) tftypes.Value {
	t.Helper()

	ctx := context.Background()
	objectType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("expected tftypes.Object, got %T", s.Type().TerraformType(ctx))
	}
	tfAttrs := make(map[string]tftypes.Value, len(objectType.AttributeTypes))

	for name, attrType := range objectType.AttributeTypes {
		if value, ok := attrs.Strings[name]; ok {
			tfValue, err := value.ToTerraformValue(ctx)
			if err != nil {
				t.Fatalf("failed to convert string %q to terraform value: %v", name, err)
			}
			tfAttrs[name] = tfValue
			continue
		}

		if value, ok := attrs.Int64s[name]; ok {
			tfValue, err := value.ToTerraformValue(ctx)
			if err != nil {
				t.Fatalf("failed to convert int64 %q to terraform value: %v", name, err)
			}
			tfAttrs[name] = tfValue
			continue
		}

		tfAttrs[name] = tftypes.NewValue(attrType, nil)
	}

	return tftypes.NewValue(objectType, tfAttrs)
}

func vmStorageAttachmentGetStateModel(t *testing.T, ctx context.Context, state tfsdk.State) vmStorageAttachmentResourceModel {
	t.Helper()

	var model vmStorageAttachmentResourceModel
	diags := state.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("state.Get diagnostics: %v", diags)
	}

	return model
}

func diskGetStateModel(t *testing.T, ctx context.Context, state tfsdk.State) diskResourceModel {
	t.Helper()

	var model diskResourceModel
	diags := state.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("state.Get diagnostics: %v", diags)
	}

	return model
}
