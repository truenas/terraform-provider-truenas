// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &UserDataSource{}

// UserDataSource implements the truenas_user data source.
type UserDataSource struct{ client *client.Client }

// NewDataSource returns a new UserDataSource.
func NewDataSource() datasource.DataSource { return &UserDataSource{} }

func (d *UserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS local user by username.",
		Attributes: map[string]dschema.Attribute{
			"id":                     dschema.Int64Attribute{Computed: true, Description: "Numeric user ID."},
			"uid":                    dschema.Int64Attribute{Computed: true, Description: "UNIX UID."},
			"username":               dschema.StringAttribute{Required: true, Description: "Username to look up."},
			"full_name":              dschema.StringAttribute{Computed: true},
			"email":                  dschema.StringAttribute{Computed: true},
			"home":                   dschema.StringAttribute{Computed: true},
			"shell":                  dschema.StringAttribute{Computed: true},
			"locked":                 dschema.BoolAttribute{Computed: true},
			"password_disabled":      dschema.BoolAttribute{Computed: true},
			"smb":                    dschema.BoolAttribute{Computed: true},
			"ssh_password_enabled":   dschema.BoolAttribute{Computed: true},
			"sshpubkey":              dschema.StringAttribute{Computed: true},
			"sudo_commands":          dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"sudo_commands_nopasswd": dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"groups":                 dschema.ListAttribute{Computed: true, ElementType: types.Int64Type},
			"group":                  dschema.Int64Attribute{Computed: true, Description: "Primary group ID for the user."},
			"builtin":                dschema.BoolAttribute{Computed: true},
			"immutable":              dschema.BoolAttribute{Computed: true},
			"local":                  dschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *UserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state UserDatasourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by username: user.query([["username", "=", "<name>"]])
	queryFilters := []any{[]any{"username", "=", state.Username.ValueString()}}
	raw, err := d.client.CallRead(ctx, "user.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query users failed", err.Error())
		return
	}

	var results []userAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"User not found",
			fmt.Sprintf("No user named %q was found.", state.Username.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
