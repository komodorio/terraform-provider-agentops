// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSkillResource(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSkillConfig(mock.URL, "deploy-runbook", "# Deploy\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("agentops_skill.test", "id"),
					resource.TestCheckResourceAttr("agentops_skill.test", "name", "deploy-runbook"),
					resource.TestCheckResourceAttr("agentops_skill.test", "content", "# Deploy\n"),
					resource.TestCheckResourceAttr("agentops_skill.test", "kind", "authored"),
					resource.TestCheckResourceAttr("agentops_skill.test", "content_version", "1"),
					resource.TestCheckResourceAttr("agentops_skill.test", "tags.0", "ops"),
					resource.TestCheckResourceAttr("agentops_skill.test", "labels.team", "core"),
				),
			},
			{
				ResourceName:      "agentops_skill.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Changing the content publishes a new version; renaming edits metadata in place.
				Config: testAccSkillConfig(mock.URL, "deploy-runbook-v2", "# Deploy v2\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_skill.test", "name", "deploy-runbook-v2"),
					resource.TestCheckResourceAttr("agentops_skill.test", "content", "# Deploy v2\n"),
					resource.TestCheckResourceAttr("agentops_skill.test", "content_version", "2"),
				),
			},
		},
	})
}

func TestAccSkillResourceWithFolder(t *testing.T) {
	mock := newMockServer(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// A skill is a folder: SKILL.md plus the files it links to. The whole folder is one
				// version, so version 1 has to carry it — not a contentless v1 with the folder in v2.
				Config: testAccSkillFolderConfig(mock.URL, "# Deploy\n", "detail v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_skill.folder", "content", "# Deploy\n"),
					resource.TestCheckResourceAttr("agentops_skill.folder", "content_version", "1"),
					resource.TestCheckResourceAttr("agentops_skill.folder", "resources.#", "2"),
					resource.TestCheckResourceAttr("agentops_skill.folder", "resources.0.path", "references/rollback.md"),
					resource.TestCheckResourceAttr("agentops_skill.folder", "resources.0.content", "detail v1"),
					resource.TestCheckResourceAttr("agentops_skill.folder", "resources.0.executable", "false"),
					resource.TestCheckResourceAttr("agentops_skill.folder", "resources.1.path", "scripts/deploy.sh"),
					resource.TestCheckResourceAttr("agentops_skill.folder", "resources.1.executable", "true"),
				),
			},
			{
				ResourceName:      "agentops_skill.folder",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// A resource-only edit is still a content change: it publishes a new version, because
				// the folder and the body are versioned together.
				Config: testAccSkillFolderConfig(mock.URL, "# Deploy\n", "detail v2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("agentops_skill.folder", "content_version", "2"),
					resource.TestCheckResourceAttr("agentops_skill.folder", "resources.0.content", "detail v2"),
				),
			},
		},
	})
}

func testAccSkillFolderConfig(endpoint, content, reference string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_skill" "folder" {
  name    = "deploy-runbook-folder"
  content = %q

  resources = [
    {
      path    = "references/rollback.md"
      content = %q
    },
    {
      path       = "scripts/deploy.sh"
      content    = "#!/bin/sh\n"
      executable = true
    },
  ]
}
`, content, reference)
}

func testAccSkillConfig(endpoint, name, content string) string {
	return mockProviderConfig(endpoint) + fmt.Sprintf(`
resource "agentops_skill" "test" {
  name        = %q
  description = "A runbook skill"
  content     = %q
  tags        = ["ops"]
  labels      = { team = "core" }
}
`, name, content)
}
