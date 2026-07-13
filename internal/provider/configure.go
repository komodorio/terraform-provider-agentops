// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
)

// clientFromProviderData extracts the configured *client.Client that the
// provider's Configure stashed in ResourceData/DataSourceData. It returns nil
// (without an error) when providerData is nil, which happens during early
// validation walks before Configure has run.
func clientFromProviderData(providerData any, diags *diag.Diagnostics) *client.Client {
	if providerData == nil {
		return nil
	}
	c, ok := providerData.(*client.Client)
	if !ok {
		diags.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *client.Client, got %T. This is a bug in the provider.", providerData),
		)
		return nil
	}
	return c
}
