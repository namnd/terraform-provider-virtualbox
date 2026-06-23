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
	Strings map[string]types.String
	Int64s  map[string]types.Int64
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

func vmGetStateModel(t *testing.T, ctx context.Context, state tfsdk.State) vmResourceModel {
	t.Helper()

	var model vmResourceModel
	diags := state.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("state.Get diagnostics: %v", diags)
	}

	return model
}
