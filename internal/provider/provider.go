package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/resources/app"
	"github.com/truenas/terraform-provider-truenas/internal/resources/dataset"
	"github.com/truenas/terraform-provider-truenas/internal/resources/group"
	"github.com/truenas/terraform-provider-truenas/internal/resources/iscsi_extent"
	"github.com/truenas/terraform-provider-truenas/internal/resources/iscsi_initiator"
	"github.com/truenas/terraform-provider-truenas/internal/resources/iscsi_target"
	"github.com/truenas/terraform-provider-truenas/internal/resources/nfs"
	"github.com/truenas/terraform-provider-truenas/internal/resources/periodic_snapshot"
	"github.com/truenas/terraform-provider-truenas/internal/resources/pool"
	"github.com/truenas/terraform-provider-truenas/internal/resources/service"
	"github.com/truenas/terraform-provider-truenas/internal/resources/smb"
	"github.com/truenas/terraform-provider-truenas/internal/resources/snapshot"
	"github.com/truenas/terraform-provider-truenas/internal/resources/user"
	"github.com/truenas/terraform-provider-truenas/internal/resources/vm"
	"github.com/truenas/terraform-provider-truenas/internal/resources/vm_device"
	"github.com/truenas/terraform-provider-truenas/internal/resources/zvol"
)

var _ provider.Provider = &TrueNASProvider{}

type TrueNASProvider struct {
	version string
}

type TrueNASProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	Insecure types.Bool   `tfsdk:"insecure"`
	CACert   types.String `tfsdk:"ca_cert"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &TrueNASProvider{version: version}
	}
}

func (p *TrueNASProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "truenas"
	resp.Version = p.version
}

func (p *TrueNASProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages TrueNAS SCALE resources via the WebSocket API.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "TrueNAS WebSocket endpoint, e.g. wss://192.168.1.68/websocket. Env: TRUENAS_ENDPOINT",
				Optional:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "TrueNAS API key. Mutually exclusive with username/password. Env: TRUENAS_API_KEY",
				Optional:    true,
				Sensitive:   true,
			},
			"username": schema.StringAttribute{
				Description: "TrueNAS username. Requires password. Env: TRUENAS_USERNAME",
				Optional:    true,
			},
			"password": schema.StringAttribute{
				Description: "TrueNAS password. Requires username. Env: TRUENAS_PASSWORD",
				Optional:    true,
				Sensitive:   true,
			},
			"insecure": schema.BoolAttribute{
				Description: "Skip TLS certificate verification. Mutually exclusive with ca_cert.",
				Optional:    true,
			},
			"ca_cert": schema.StringAttribute{
				Description: "Path to a PEM-encoded CA certificate file. Mutually exclusive with insecure.",
				Optional:    true,
			},
		},
	}
}

func (p *TrueNASProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg TrueNASProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Env var fallback
	endpoint := envOrVal(cfg.Endpoint, "TRUENAS_ENDPOINT")
	apiKey := envOrVal(cfg.APIKey, "TRUENAS_API_KEY")
	username := envOrVal(cfg.Username, "TRUENAS_USERNAME")
	password := envOrVal(cfg.Password, "TRUENAS_PASSWORD")

	if endpoint == "" {
		resp.Diagnostics.AddError("Missing endpoint", "Set endpoint in provider config or TRUENAS_ENDPOINT env var.")
		return
	}

	// Auth validation: exactly one auth method
	hasAPIKey := apiKey != ""
	hasPassword := username != "" && password != ""
	if hasAPIKey && hasPassword {
		resp.Diagnostics.AddError("Conflicting auth", "Provide api_key OR username+password, not both.")
		return
	}
	if !hasAPIKey && !hasPassword {
		resp.Diagnostics.AddError("Missing auth", "Provide api_key or username+password.")
		return
	}

	// TLS validation: insecure and ca_cert are mutually exclusive
	insecure := !cfg.Insecure.IsNull() && !cfg.Insecure.IsUnknown() && cfg.Insecure.ValueBool()
	caFile := ""
	if !cfg.CACert.IsNull() && !cfg.CACert.IsUnknown() {
		caFile = cfg.CACert.ValueString()
	}
	if insecure && caFile != "" {
		resp.Diagnostics.AddError("Conflicting TLS config", "Set insecure or ca_cert, not both.")
		return
	}

	tlsCfg, err := client.BuildTLSConfig(insecure, caFile)
	if err != nil {
		resp.Diagnostics.AddError("TLS config error", err.Error())
		return
	}

	c := client.New(endpoint, tlsCfg)

	var authFn func(ctx context.Context) error
	if hasAPIKey {
		key := apiKey
		authFn = func(ctx context.Context) error { return client.AuthAPIKey(ctx, c, key) }
	} else {
		u, pw := username, password
		authFn = func(ctx context.Context) error { return client.AuthPassword(ctx, c, u, pw) }
	}

	if err := c.Connect(ctx, authFn); err != nil {
		resp.Diagnostics.AddError("Connection failed", fmt.Sprintf("Cannot connect to TrueNAS at %s: %s", endpoint, err))
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *TrueNASProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		app.NewResource,
		dataset.NewResource,
		group.NewResource,
		iscsi_extent.NewResource,
		iscsi_initiator.NewResource,
		iscsi_target.NewResource,
		nfs.NewResource,
		periodic_snapshot.NewResource,
		pool.NewResource,
		service.NewResource,
		smb.NewResource,
		snapshot.NewResource,
		user.NewResource,
		vm.NewResource,
		vm_device.NewResource,
		zvol.NewResource,
	}
}

func (p *TrueNASProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		app.NewDataSource,
		dataset.NewDataSource,
		group.NewDataSource,
		iscsi_extent.NewDataSource,
		iscsi_initiator.NewDataSource,
		iscsi_target.NewDataSource,
		nfs.NewDataSource,
		periodic_snapshot.NewDataSource,
		pool.NewDataSource,
		service.NewDataSource,
		smb.NewDataSource,
		snapshot.NewDataSource,
		user.NewDataSource,
		vm.NewDataSource,
		vm_device.NewDataSource,
		zvol.NewDataSource,
	}
}

// envOrVal returns the string value of a types.String, falling back to an env var.
func envOrVal(v types.String, envKey string) string {
	if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		return v.ValueString()
	}
	return os.Getenv(envKey)
}
