// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package s3_config

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// AuditActions are the S3 operations default_audit may name (s3.update).
var AuditActions = []string{"GetObject", "PutObject", "DeleteObject", "GetObjectTagging", "PutObjectTagging",
	"DeleteObjectTagging", "ListBucket", "GetBucketLocation", "ListBucketMultipartUploads", "ListMultipartUploadParts",
	"AbortMultipartUpload", "PutObjectRetention", "PutObjectLegalHold", "ListAllMyBuckets", "GetObjectAcl",
	"PutObjectAcl", "GetBucketAcl", "PutBucketAcl", "PutBucketVersioning", "CreateBucket", "DeleteBucket", AuditAll}

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS S3 service configuration (s3.config / s3.update; TrueNAS 27.0 and later). " +
			"This is a singleton resource: there is exactly one S3 configuration per TrueNAS system, so it is never " +
			"created or deleted on TrueNAS; Terraform create/update call s3.update, and Terraform delete only removes " +
			"the resource from state. Buckets have no TrueNAS API: they are created through the S3 protocol " +
			"(CreateBucket) by an access key with manage_buckets, under managed_root_dataset. Turn the service on with " +
			"truenas_service (\"s3\"). See the \"S3 state backend\" guide.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"s3_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"listeners": schema.ListNestedAttribute{
				Optional: true,
				Computed: true,
				Description: "Where the S3 service listens, at most eight entries. An empty list listens on every address on " +
					"port 9000 in plaintext. Changing it restarts the service.",
				Validators:    []validator.List{listvalidator.SizeAtMost(8)},
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"address": schema.StringAttribute{
							Required:    true,
							Description: "IP address to listen on (s3.bindip_choices): a static address, a loopback or a wildcard (0.0.0.0, ::).",
						},
						"port": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Description: "TCP port to listen on. Default 9000.",
						},
						"tls": schema.BoolAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Serve this address over TLS with `certificate` (which must then be set). Default false.",
						},
					},
				},
			},
			"servers": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Reactor threads serving the listen addresses (1-12).",
				Validators:    []validator.Int64{int64validator.Between(1, 12)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"certificate": schema.Int64Attribute{
				Optional: true,
				Description: "ID of the certificate the TLS listeners serve (truenas_certificate.x.id). Unset (null) serves them " +
					"with the UI certificate; removing it from the configuration clears a certificate set before.",
			},
			"region": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Region name echoed to clients, us-east-1 shaped (lowercase letters, digits, hyphens; at most 63). Empty accepts whatever a client sends.",
				Validators:    []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z0-9-]{0,63}$`), "lowercase letters, digits and hyphens, at most 63")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"log_level": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Least serious log record the S3 service keeps: ERROR, WARNING, NOTICE, INFO (one record per request) or DEBUG.",
				Validators:    []validator.String{stringvalidator.OneOf("ERROR", "WARNING", "NOTICE", "INFO", "DEBUG")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"default_audit": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: stringTypeAlias(),
				Description: "S3 actions audited on every bucket that does not set its own audit list, e.g. [\"PutObject\", \"DeleteObject\"]. " +
					"[\"ALL\"] audits everything (the API's \"ALL\"); an empty list audits nothing.",
				Validators:    []validator.List{listvalidator.ValueStringsAre(stringvalidator.OneOf(AuditActions...))},
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"default_audit_overflow": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "What an audited request gets when no audit record slot is free: DROP sheds the record, BACKPRESSURE delays the request.",
				Validators:    []validator.String{stringvalidator.OneOf("DROP", "BACKPRESSURE")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"global_grants": schema.ListNestedAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Grants that apply to every bucket; the list replaces the current one.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"principal_type": schema.StringAttribute{
							Required:    true,
							Description: "USER, GROUP or EVERYONE (everyone with a valid access key).",
							Validators:  []validator.String{stringvalidator.OneOf("USER", "GROUP", "EVERYONE")},
						},
						"xid": schema.Int64Attribute{
							Optional:    true,
							Description: "uid of the user or gid of the group. Required for USER and GROUP, forbidden for EVERYONE.",
						},
						"access": schema.StringAttribute{
							Required:    true,
							Description: "READONLY, WRITEONLY, READWRITE, or DENY (refuses every operation, outranks the bucket owner).",
							Validators:  []validator.String{stringvalidator.OneOf("READONLY", "WRITEONLY", "READWRITE", "DENY")},
						},
					},
				},
			},
			"managed_root_dataset": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Dataset under which buckets created through the S3 protocol get their datasets, e.g. tank/s3. " +
					"Null (or empty) means buckets cannot be created through S3.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
