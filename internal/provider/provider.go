// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/resources/acl_template"
	"github.com/truenas/terraform-provider-truenas/internal/resources/acme_dns_authenticator"
	"github.com/truenas/terraform-provider-truenas/internal/resources/alert_policy"
	"github.com/truenas/terraform-provider-truenas/internal/resources/alert_service"
	"github.com/truenas/terraform-provider-truenas/internal/resources/api_key"
	"github.com/truenas/terraform-provider-truenas/internal/resources/app"
	"github.com/truenas/terraform-provider-truenas/internal/resources/app_registry"
	"github.com/truenas/terraform-provider-truenas/internal/resources/audit_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/boot_environment"
	"github.com/truenas/terraform-provider-truenas/internal/resources/catalog_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/certificate"
	"github.com/truenas/terraform-provider-truenas/internal/resources/cloud_backup"
	"github.com/truenas/terraform-provider-truenas/internal/resources/cloudsync"
	"github.com/truenas/terraform-provider-truenas/internal/resources/cloudsync_credentials"
	"github.com/truenas/terraform-provider-truenas/internal/resources/container"
	"github.com/truenas/terraform-provider-truenas/internal/resources/container_device"
	"github.com/truenas/terraform-provider-truenas/internal/resources/container_image"
	"github.com/truenas/terraform-provider-truenas/internal/resources/cronjob"
	"github.com/truenas/terraform-provider-truenas/internal/resources/dataset"
	"github.com/truenas/terraform-provider-truenas/internal/resources/directoryservices"
	"github.com/truenas/terraform-provider-truenas/internal/resources/docker_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/docker_network"
	"github.com/truenas/terraform-provider-truenas/internal/resources/enclosure"
	"github.com/truenas/terraform-provider-truenas/internal/resources/enclosure_label"
	"github.com/truenas/terraform-provider-truenas/internal/resources/failover_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/filesystem_acl"
	"github.com/truenas/terraform-provider-truenas/internal/resources/filesystem_permissions"
	"github.com/truenas/terraform-provider-truenas/internal/resources/ftp_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/group"
	"github.com/truenas/terraform-provider-truenas/internal/resources/init_shutdown_script"
	"github.com/truenas/terraform-provider-truenas/internal/resources/ipmi_lan"
	"github.com/truenas/terraform-provider-truenas/internal/resources/iscsi_auth"
	"github.com/truenas/terraform-provider-truenas/internal/resources/iscsi_extent"
	"github.com/truenas/terraform-provider-truenas/internal/resources/iscsi_global"
	"github.com/truenas/terraform-provider-truenas/internal/resources/iscsi_initiator"
	"github.com/truenas/terraform-provider-truenas/internal/resources/iscsi_portal"
	"github.com/truenas/terraform-provider-truenas/internal/resources/iscsi_target"
	"github.com/truenas/terraform-provider-truenas/internal/resources/iscsi_targetextent"
	"github.com/truenas/terraform-provider-truenas/internal/resources/kerberos_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/kerberos_keytab"
	"github.com/truenas/terraform-provider-truenas/internal/resources/kerberos_realm"
	"github.com/truenas/terraform-provider-truenas/internal/resources/keychain_ssh_connection"
	"github.com/truenas/terraform-provider-truenas/internal/resources/keychain_ssh_keypair"
	"github.com/truenas/terraform-provider-truenas/internal/resources/lxc_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/mail"
	"github.com/truenas/terraform-provider-truenas/internal/resources/network_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/network_interface"
	"github.com/truenas/terraform-provider-truenas/internal/resources/nfs"
	"github.com/truenas/terraform-provider-truenas/internal/resources/nfs_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/ntp_server"
	"github.com/truenas/terraform-provider-truenas/internal/resources/nvmet_global"
	"github.com/truenas/terraform-provider-truenas/internal/resources/nvmet_host"
	"github.com/truenas/terraform-provider-truenas/internal/resources/nvmet_host_subsys"
	"github.com/truenas/terraform-provider-truenas/internal/resources/nvmet_namespace"
	"github.com/truenas/terraform-provider-truenas/internal/resources/nvmet_port"
	"github.com/truenas/terraform-provider-truenas/internal/resources/nvmet_port_subsys"
	"github.com/truenas/terraform-provider-truenas/internal/resources/nvmet_subsys"
	"github.com/truenas/terraform-provider-truenas/internal/resources/periodic_snapshot"
	"github.com/truenas/terraform-provider-truenas/internal/resources/pool"
	"github.com/truenas/terraform-provider-truenas/internal/resources/privilege"
	"github.com/truenas/terraform-provider-truenas/internal/resources/replication"
	"github.com/truenas/terraform-provider-truenas/internal/resources/replication_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/reporting_exporter"
	"github.com/truenas/terraform-provider-truenas/internal/resources/resilver_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/rsync_task"
	"github.com/truenas/terraform-provider-truenas/internal/resources/scrub_task"
	"github.com/truenas/terraform-provider-truenas/internal/resources/service"
	"github.com/truenas/terraform-provider-truenas/internal/resources/smb"
	"github.com/truenas/terraform-provider-truenas/internal/resources/smb_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/snapshot"
	"github.com/truenas/terraform-provider-truenas/internal/resources/snmp_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/ssh_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/static_route"
	"github.com/truenas/terraform-provider-truenas/internal/resources/system_advanced"
	"github.com/truenas/terraform-provider-truenas/internal/resources/system_dataset"
	"github.com/truenas/terraform-provider-truenas/internal/resources/system_general"
	"github.com/truenas/terraform-provider-truenas/internal/resources/tn_connect_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/truecommand_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/tunable"
	"github.com/truenas/terraform-provider-truenas/internal/resources/twofactor_auth"
	"github.com/truenas/terraform-provider-truenas/internal/resources/ups_config"
	"github.com/truenas/terraform-provider-truenas/internal/resources/user"
	"github.com/truenas/terraform-provider-truenas/internal/resources/vm"
	"github.com/truenas/terraform-provider-truenas/internal/resources/vm_device"
	"github.com/truenas/terraform-provider-truenas/internal/resources/vmware"
	"github.com/truenas/terraform-provider-truenas/internal/resources/webshare"
	"github.com/truenas/terraform-provider-truenas/internal/resources/webshare_config"
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
		Description: "Manages TrueNAS resources via the WebSocket API.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "TrueNAS WebSocket endpoint, e.g. wss://truenas.example.com/api/current (legacy /websocket paths are rewritten to /api/current automatically). Env: TRUENAS_ENDPOINT",
				Optional:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "TrueNAS API key. Mutually exclusive with password auth. On TrueNAS 26.0+, also set username (the key owner) to authenticate via SCRAM-SHA-512 so the raw key never crosses the wire. Env: TRUENAS_API_KEY",
				Optional:    true,
				Sensitive:   true,
			},
			"username": schema.StringAttribute{
				Description: "TrueNAS username. With password: password authentication. With api_key: names the key owner and enables SCRAM-SHA-512 on TrueNAS 26.0+. Env: TRUENAS_USERNAME",
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

	// The client speaks JSON-RPC 2.0 (/api/current). Endpoints written for
	// the legacy DDP path are rewritten transparently so existing
	// configurations keep working.
	if base, ok := strings.CutSuffix(endpoint, "/websocket"); ok {
		endpoint = base + "/api/current"
	}

	// Auth validation: exactly one auth method. username may accompany
	// api_key — it names the key's owner and enables SCRAM (26.0+).
	hasAPIKey := apiKey != ""
	hasPassword := username != "" && password != ""
	if hasAPIKey && password != "" {
		resp.Diagnostics.AddError("Conflicting auth", "Provide api_key OR username+password, not both. (username alone may accompany api_key to enable SCRAM.)")
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

	c := client.New(endpoint, tlsCfg, client.WithUserAgent(p.version))

	var authFn func(ctx context.Context) error
	if hasAPIKey {
		// AuthAPIKeyAuto upgrades to SCRAM-SHA-512 when the server offers
		// it (TrueNAS 26.0+) and username identifies the key owner;
		// otherwise (or when username is unset) it uses the plain login.
		key, u := apiKey, username
		authFn = func(ctx context.Context) error { return client.AuthAPIKeyAuto(ctx, c, u, key) }
	} else {
		u, pw := username, password
		authFn = func(ctx context.Context) error { return client.AuthPassword(ctx, c, u, pw) }
	}

	retryingAuthFn := func(ctx context.Context) error { return client.WithRetry(ctx, authFn) }

	if err := c.Connect(ctx, retryingAuthFn); err != nil {
		resp.Diagnostics.AddError("Connection failed", fmt.Sprintf("Cannot connect to TrueNAS at %s: %s", endpoint, err))
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *TrueNASProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		acl_template.NewResource,
		acme_dns_authenticator.NewResource,
		alert_policy.NewResource,
		alert_service.NewResource,
		api_key.NewResource,
		app.NewResource,
		app_registry.NewResource,
		audit_config.NewResource,
		boot_environment.NewResource,
		catalog_config.NewResource,
		certificate.NewResource,
		cloud_backup.NewResource,
		cloudsync.NewResource,
		cloudsync_credentials.NewResource,
		container.NewResource,
		container_device.NewResource,
		cronjob.NewResource,
		dataset.NewResource,
		directoryservices.NewResource,
		docker_config.NewResource,
		enclosure_label.NewResource,
		failover_config.NewResource,
		filesystem_acl.NewResource,
		filesystem_permissions.NewResource,
		ftp_config.NewResource,
		group.NewResource,
		init_shutdown_script.NewResource,
		ipmi_lan.NewResource,
		iscsi_auth.NewResource,
		iscsi_extent.NewResource,
		iscsi_global.NewResource,
		iscsi_initiator.NewResource,
		iscsi_portal.NewResource,
		iscsi_target.NewResource,
		iscsi_targetextent.NewResource,
		kerberos_config.NewResource,
		kerberos_keytab.NewResource,
		kerberos_realm.NewResource,
		keychain_ssh_connection.NewResource,
		keychain_ssh_keypair.NewResource,
		lxc_config.NewResource,
		mail.NewResource,
		nfs.NewResource,
		nfs_config.NewResource,
		network_config.NewResource,
		network_interface.NewResource,
		ntp_server.NewResource,
		nvmet_global.NewResource,
		nvmet_host.NewResource,
		nvmet_host_subsys.NewResource,
		nvmet_namespace.NewResource,
		nvmet_port.NewResource,
		nvmet_port_subsys.NewResource,
		nvmet_subsys.NewResource,
		periodic_snapshot.NewResource,
		pool.NewResource,
		privilege.NewResource,
		replication.NewResource,
		replication_config.NewResource,
		reporting_exporter.NewResource,
		resilver_config.NewResource,
		rsync_task.NewResource,
		scrub_task.NewResource,
		service.NewResource,
		smb.NewResource,
		smb_config.NewResource,
		snapshot.NewResource,
		snmp_config.NewResource,
		static_route.NewResource,
		ssh_config.NewResource,
		system_advanced.NewResource,
		system_dataset.NewResource,
		system_general.NewResource,
		tn_connect_config.NewResource,
		truecommand_config.NewResource,
		tunable.NewResource,
		twofactor_auth.NewResource,
		user.NewResource,
		ups_config.NewResource,
		vm.NewResource,
		vm_device.NewResource,
		vmware.NewResource,
		webshare.NewResource,
		webshare_config.NewResource,
		zvol.NewResource,
	}
}

func (p *TrueNASProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		acl_template.NewDataSource,
		acme_dns_authenticator.NewDataSource,
		alert_policy.NewDataSource,
		alert_service.NewDataSource,
		api_key.NewDataSource,
		app.NewDataSource,
		app_registry.NewDataSource,
		audit_config.NewDataSource,
		boot_environment.NewDataSource,
		catalog_config.NewDataSource,
		certificate.NewDataSource,
		cloud_backup.NewDataSource,
		cloudsync.NewDataSource,
		cloudsync_credentials.NewDataSource,
		container.NewDataSource,
		container_device.NewDataSource,
		container_image.NewDataSource, // datasource-only: no truenas_container_image resource exists (see internal/resources/container_image/schema.go)
		cronjob.NewDataSource,
		dataset.NewDataSource,
		directoryservices.NewDataSource,
		docker_config.NewDataSource,
		docker_network.NewDataSource, // datasource-only: no truenas_docker_network resource exists (see internal/resources/docker_network/schema.go)
		enclosure.NewDataSource,      // read-only enclosure lookup; label management lives in truenas_enclosure_label (see internal/resources/enclosure/schema.go)
		failover_config.NewDataSource,
		filesystem_acl.NewDataSource,
		filesystem_permissions.NewDataSource,
		ftp_config.NewDataSource,
		group.NewDataSource,
		init_shutdown_script.NewDataSource,
		ipmi_lan.NewDataSource,
		iscsi_auth.NewDataSource,
		iscsi_extent.NewDataSource,
		iscsi_global.NewDataSource,
		iscsi_initiator.NewDataSource,
		iscsi_portal.NewDataSource,
		iscsi_target.NewDataSource,
		iscsi_targetextent.NewDataSource,
		kerberos_config.NewDataSource,
		kerberos_keytab.NewDataSource,
		kerberos_realm.NewDataSource,
		keychain_ssh_connection.NewDataSource,
		keychain_ssh_keypair.NewDataSource,
		lxc_config.NewDataSource,
		mail.NewDataSource,
		nfs.NewDataSource,
		nfs_config.NewDataSource,
		network_config.NewDataSource,
		network_interface.NewDataSource,
		ntp_server.NewDataSource,
		nvmet_global.NewDataSource,
		nvmet_host.NewDataSource,
		nvmet_host_subsys.NewDataSource,
		nvmet_namespace.NewDataSource,
		nvmet_port.NewDataSource,
		nvmet_port_subsys.NewDataSource,
		nvmet_subsys.NewDataSource,
		periodic_snapshot.NewDataSource,
		pool.NewDataSource,
		privilege.NewDataSource,
		replication.NewDataSource,
		replication_config.NewDataSource,
		reporting_exporter.NewDataSource,
		resilver_config.NewDataSource,
		rsync_task.NewDataSource,
		scrub_task.NewDataSource,
		service.NewDataSource,
		smb.NewDataSource,
		smb_config.NewDataSource,
		snapshot.NewDataSource,
		snmp_config.NewDataSource,
		static_route.NewDataSource,
		ssh_config.NewDataSource,
		system_advanced.NewDataSource,
		system_dataset.NewDataSource,
		system_general.NewDataSource,
		tn_connect_config.NewDataSource,
		truecommand_config.NewDataSource,
		tunable.NewDataSource,
		twofactor_auth.NewDataSource,
		user.NewDataSource,
		ups_config.NewDataSource,
		vm.NewDataSource,
		vm_device.NewDataSource,
		vmware.NewDataSource,
		webshare.NewDataSource,
		webshare_config.NewDataSource,
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
