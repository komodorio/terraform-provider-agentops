// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stringToPtr converts a Terraform string into a *string suitable for a request
// body: null/unknown values become nil so the field is omitted.
func stringToPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// boolToPtr is the bool analogue of stringToPtr.
func boolToPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// int64ToIntPtr converts a Terraform int64 into a *int request field, mapping
// null/unknown to nil so the field is omitted.
func int64ToIntPtr(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := int(v.ValueInt64())
	return &i
}

// ptrToString converts an optional API string into a Terraform string, mapping
// nil to null.
func ptrToString(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// boolPtrToBool converts an optional API bool into a Terraform bool, mapping nil
// to null.
func boolPtrToBool(p *bool) types.Bool {
	if p == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*p)
}

// strOrNull maps an empty string to null and any other value to itself. Use it
// for optional API fields that arrive as a bare "" when unset.
func strOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// enumPtrToString renders a pointer to any string-backed enum as a plain string
// ("" when nil).
func enumPtrToString[T ~string](p *T) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

// stringMapToPtr converts an optional Terraform map into a *map[string]string
// request field, leaving it nil (omitted) when the map is null or unknown.
func stringMapToPtr(ctx context.Context, m types.Map, target **map[string]string) diag.Diagnostics {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	out := map[string]string{}
	diags := m.ElementsAs(ctx, &out, false)
	if diags.HasError() {
		return diags
	}
	*target = &out
	return diags
}

// stringMapValue maps an optional API string map to a Terraform map, mapping nil
// to a null map.
func stringMapValue(ctx context.Context, p *map[string]string) (types.Map, diag.Diagnostics) {
	if p == nil {
		return types.MapNull(types.StringType), nil
	}
	return types.MapValueFrom(ctx, types.StringType, *p)
}

// boolMapToPtr converts an optional Terraform map of booleans into a
// *map[string]bool request field, leaving it nil (omitted) when null or unknown.
func boolMapToPtr(ctx context.Context, m types.Map, target **map[string]bool) diag.Diagnostics {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	out := map[string]bool{}
	diags := m.ElementsAs(ctx, &out, false)
	if diags.HasError() {
		return diags
	}
	*target = &out
	return diags
}

// jsonToMapPtr unmarshals a normalized JSON-string attribute into the free-form
// map expected by a request body, leaving it nil (omitted) when null or unknown.
func jsonToMapPtr(v jsontypes.Normalized, target **map[string]interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() {
		return diags
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(v.ValueString()), &m); err != nil {
		diags.AddError("Invalid JSON attribute", "Failed to parse a JSON object attribute: "+err.Error())
		return diags
	}
	*target = &m
	return diags
}

// mapPtrToJSON renders an optional free-form map from a response back into a
// normalized JSON-string attribute, mapping a nil/empty map to null.
func mapPtrToJSON(m *map[string]interface{}) jsontypes.Normalized {
	if m == nil || len(*m) == 0 {
		return jsontypes.NewNormalizedNull()
	}
	b, err := json.Marshal(*m)
	if err != nil {
		return jsontypes.NewNormalizedNull()
	}
	return jsontypes.NewNormalizedValue(string(b))
}

// listToStringSlice converts an optional Terraform list into a *[]string request
// field, leaving it nil (omitted) when the list is null or unknown.
func listToStringSlice(ctx context.Context, list types.List, target **[]string) diag.Diagnostics {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var s []string
	diags := list.ElementsAs(ctx, &s, false)
	if diags.HasError() {
		return diags
	}
	*target = &s
	return diags
}
