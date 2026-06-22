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

func vmTestPlan(t *testing.T, schema schema.Schema, attrs map[string]types.String) tfsdk.Plan {
	t.Helper()

	return tfsdk.Plan{
		Schema: schema,
		Raw:    vmTestObjectValue(t, schema, attrs),
	}
}

func vmTestState(t *testing.T, schema schema.Schema, attrs map[string]types.String) tfsdk.State {
	t.Helper()

	return tfsdk.State{
		Schema: schema,
		Raw:    vmTestObjectValue(t, schema, attrs),
	}
}

func vmTestObjectValue(t *testing.T, s schema.Schema, attrs map[string]types.String) tftypes.Value {
	t.Helper()

	ctx := context.Background()
	objectType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("expected tftypes.Object, got %T", s.Type().TerraformType(ctx))
	}
	tfAttrs := make(map[string]tftypes.Value, len(objectType.AttributeTypes))

	for name, attrType := range objectType.AttributeTypes {
		value, ok := attrs[name]
		if !ok {
			tfAttrs[name] = tftypes.NewValue(attrType, nil)
			continue
		}

		tfValue, err := value.ToTerraformValue(ctx)
		if err != nil {
			t.Fatalf("failed to convert %q to terraform value: %v", name, err)
		}
		tfAttrs[name] = tfValue
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
