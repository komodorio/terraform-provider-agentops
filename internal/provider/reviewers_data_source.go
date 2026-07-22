// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/komodorio/terraform-provider-agentops/internal/client"
)

var (
	_ datasource.DataSource              = &reviewersDataSource{}
	_ datasource.DataSourceWithConfigure = &reviewersDataSource{}
)

// NewReviewersDataSource is the constructor registered with the provider.
func NewReviewersDataSource() datasource.DataSource {
	return &reviewersDataSource{}
}

type reviewersDataSource struct {
	client *client.Client
}

type reviewersDataSourceModel struct {
	Reviewers []reviewerSummaryModel `tfsdk:"reviewers"`
}

type reviewerSummaryModel struct {
	AgentID       types.String `tfsdk:"agent_id"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	IsBuiltin     types.Bool   `tfsdk:"is_builtin"`
	WorkflowCount types.Int64  `tfsdk:"workflow_count"`
	Reviews30d    types.Int64  `tfsdk:"reviews_30d"`
	Findings30d   types.Int64  `tfsdk:"findings_30d"`
	LastReviewAt  types.String `tfsdk:"last_review_at"`
}

func (d *reviewersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reviewers"
}

func (d *reviewersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The reviewer agents available to this account, including built-in reviewers, " +
			"for use in `agentops_review_workflow.reviewer_agent_ids`.",
		Attributes: map[string]schema.Attribute{
			"reviewers": schema.ListNestedAttribute{
				MarkdownDescription: "Available reviewer agents.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"agent_id":       schema.StringAttribute{Computed: true, MarkdownDescription: "Reviewer agent identifier."},
						"name":           schema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
						"description":    schema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
						"is_builtin":     schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether this is a built-in reviewer."},
						"workflow_count": schema.Int64Attribute{Computed: true, MarkdownDescription: "Number of workflows using this reviewer."},
						"reviews_30d":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Reviews performed in the last 30 days."},
						"findings_30d":   schema.Int64Attribute{Computed: true, MarkdownDescription: "Findings raised in the last 30 days."},
						"last_review_at": schema.StringAttribute{Computed: true, MarkdownDescription: "Timestamp of the most recent review."},
					},
				},
			},
		},
	}
}

func (d *reviewersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *reviewersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	apiResp, err := d.client.Gen.ReviewersListReviewersEndpointWithResponse(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading reviewers", err.Error())
		return
	}
	if err := client.Check(apiResp.HTTPResponse, apiResp.Body); err != nil {
		resp.Diagnostics.AddError("Error reading reviewers", err.Error())
		return
	}
	if apiResp.JSON200 == nil {
		return
	}

	var state reviewersDataSourceModel
	for _, e := range *apiResp.JSON200 {
		state.Reviewers = append(state.Reviewers, reviewerSummaryModel{
			AgentID:       types.StringValue(e.AgentId),
			Name:          types.StringValue(e.Name),
			Description:   ptrToString(e.Description),
			IsBuiltin:     types.BoolValue(e.IsBuiltin),
			WorkflowCount: types.Int64Value(int64(e.WorkflowCount)),
			Reviews30d:    types.Int64Value(int64(e.Reviews30d)),
			Findings30d:   types.Int64Value(int64(e.Findings30d)),
			LastReviewAt:  ptrToString(e.LastReviewAt),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
