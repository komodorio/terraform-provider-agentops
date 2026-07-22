// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccReviewWorkflowResource covers full CRUD + import, the repos list, the
// computed repo_status projection, and the draft/active/paused lifecycle.
func TestAccReviewWorkflowResource(t *testing.T) {
	mock := newMockServer(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccReviewWorkflowConfig(mock.URL, "active"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_review_workflow.test", "id"),
					resource.TestCheckResourceAttr("agentops_review_workflow.test", "name", "pr-reviews"),
					resource.TestCheckResourceAttr("agentops_review_workflow.test", "status", "active"),
					resource.TestCheckResourceAttr("agentops_review_workflow.test", "reviewer_agent_ids.0", "agent_rev_1"),
					resource.TestCheckResourceAttr("agentops_review_workflow.test", "repos.0.repo_owner", "komodorio"),
					resource.TestCheckResourceAttr("agentops_review_workflow.test", "repos.0.repo_name", "mono"),
					resource.TestCheckResourceAttr("agentops_review_workflow.test", "repo_count", "1"),
					resource.TestCheckResourceAttr("agentops_review_workflow.test", "repo_status.0.webhook_status", "active"),
					resource.TestCheckResourceAttrSet("agentops_review_workflow.test", "webhook_url"),
				),
			},
			{
				ResourceName:      "agentops_review_workflow.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccReviewWorkflowConfig(mock.URL, "paused"),
				Check:  resource.TestCheckResourceAttr("agentops_review_workflow.test", "status", "paused"),
			},
		},
	})
}

// TestAccReviewWorkflowResource_configChangeForcesReplace verifies that changing a
// config-bearing attribute on a published (active) workflow forces replacement rather than
// an in-place update. The API only accepts config updates while a workflow is draft, so an
// in-place PATCH of an active workflow would fail (the mock rejects it); a successful apply
// proves the change went through destroy+recreate (which also pauses before deleting).
func TestAccReviewWorkflowResource_configChangeForcesReplace(t *testing.T) {
	mock := newMockServer(t)

	config := func(branch string) string {
		return mockProviderConfig(mock.URL) + fmt.Sprintf(`
resource "agentops_review_workflow" "repl" {
  name               = "pr-reviews-repl"
  status             = "active"
  base_branch_filter = %q
  reviewer_agent_ids = ["agent_rev_1"]

  repos = [
    {
      repo_owner = "komodorio"
      repo_name  = "mono"
    },
  ]
}
`, branch)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("main"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_review_workflow.repl", "status", "active"),
					resource.TestCheckResourceAttr("agentops_review_workflow.repl", "base_branch_filter", "main"),
				),
			},
			{
				// Changing base_branch_filter on the active workflow must recreate it and re-activate.
				Config: config("develop"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_review_workflow.repl", "status", "active"),
					resource.TestCheckResourceAttr("agentops_review_workflow.repl", "base_branch_filter", "develop"),
				),
			},
		},
	})
}

func testAccReviewWorkflowConfig(endpoint, status string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_review_workflow" "test" {
  name               = "pr-reviews"
  status             = %q
  reviewer_agent_ids = ["agent_rev_1"]

  repos = [
    {
      repo_owner = "komodorio"
      repo_name  = "mono"
    },
  ]
}
`, status)
}
