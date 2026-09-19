// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acl_template

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &AclTemplateDataSource{}

// AclTemplateDataSource implements the truenas_acl_template data source.
type AclTemplateDataSource struct{ client *client.Client }

// NewDataSource returns a new AclTemplateDataSource.
func NewDataSource() datasource.DataSource { return &AclTemplateDataSource{} }

func (d *AclTemplateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_acl_template"
}

func (d *AclTemplateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a filesystem ACL template (builtin or user-created) by name.",
		Attributes: map[string]dschema.Attribute{
			"id":      dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the ACL template."},
			"name":    dschema.StringAttribute{Required: true, Description: "Template name to look up."},
			"acltype": dschema.StringAttribute{Computed: true, Description: "ACL type this template provides (NFS4 or POSIX1E)."},
			"acl":     dschema.StringAttribute{Computed: true, Description: "JSON array of Access Control Entries. See the resource schema for the exact shape."},
			"comment": dschema.StringAttribute{Computed: true, Description: "Descriptive comment about the template's purpose."},
			"builtin": dschema.BoolAttribute{Computed: true, Description: "Whether this is one of TrueNAS's built-in system templates."},
		},
	}
}

func (d *AclTemplateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *AclTemplateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state AclTemplateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "filesystem.acltemplate.query",
		[]any{[]any{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query ACL templates failed", err.Error())
		return
	}

	var results []aclTemplateAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"ACL template not found",
			fmt.Sprintf("No ACL template named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
