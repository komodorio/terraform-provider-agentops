// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

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
