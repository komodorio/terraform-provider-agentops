// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCapabilitiesDataSource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock.URL) + `data "agentops_capabilities" "test" {}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.agentops_capabilities.test", "capabilities.0.key", "agent.invoke"),
				resource.TestCheckResourceAttr("data.agentops_capabilities.test", "capabilities.0.domain", "agents"),
			),
		}},
	})
}
