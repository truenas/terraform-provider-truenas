# Automated Test Inventory

Every automated test function in the repository, grouped by package and tagged by tier/gate. Generated from the `*_test.go` sources by `scripts/build-test-inventory.py` (`make test-inventory`) — regenerate after adding or renaming tests. See `TEST-PLAN.md` for the testing strategy and `TESTING.md` for how to run each tier.

**Total: 1253 test functions** across 90 packages.

| Tier / gate | Count |
|---|---|
| Acceptance · Apps gate | 2 |
| Acceptance · DS gate | 3 |
| Acceptance · HA gate | 7 |
| Acceptance · Tier 1 | 54 |
| Acceptance · Tier 2 (disruptive) | 39 |
| Acceptance · conditional skip | 22 |
| Live (client) | 10 |
| Unit | 1116 |

Tier legend: **Unit** needs no server (pure functions). **Acceptance · Tier 1** creates and destroys its own objects. **Tier 2 (disruptive)** mutates a singleton and restores it. **DS / HA / Apps gate** needs a directory server / HA system / app catalog and a matching env gate. **conditional skip** self-skips when a required fixture/endpoint/env is absent — including the permanently skipped tests whose `create` validates against a real remote endpoint (app_registry, cloud_backup, vmware), each with re-enable steps in the test file. **Live (client)** exercises the WebSocket client against a real box.

---

## `internal/acctest`  (3)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestEndpoint` | Unit |  |
| `TestRandName` | Unit |  |
| `TestTestPool` | Unit |  |

## `internal/client`  (24)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestLiveAuthPassword` | Live (client) | TestLiveAuthPassword: password login against the real box. |
| `TestLiveAuth_BadKeyRejected` | Live (client) | TestLiveAuth_BadKeyRejected: the real middleware rejects a malformed key. |
| `TestLiveCallJob_SyncIntBailout` | Live (client) | TestLiveCallJob_SyncIntBailout: regression coverage for the CallJob infinite-poll bug, against the real box. |
| `TestLiveCallRead_ReconnectsAfterTransportDrop` | Live (client) | TestLiveCallRead_ReconnectsAfterTransportDrop force-closes the live connection out from under the client and asserts CallRead re-dials the real box and succeeds. |
| `TestLiveCall_ContextCancel` | Live (client) | TestLiveCall_ContextCancel: a cancelled context aborts the call. |
| `TestLiveCall_NotFound` | Live (client) | TestLiveCall_NotFound: the real middleware's InstanceNotFound error maps to IsNotFound (errname EINVAL with an "[ENOENT]" reason on the wire). |
| `TestLiveCall_Success` | Live (client) | TestLiveCall_Success: a plain synchronous call round-trips. |
| `TestLiveReconnect_NoOpWhenAlreadyConnected` | Live (client) | TestLiveReconnect_NoOpWhenAlreadyConnected: reconnect() must not re-dial over a live connection. |
| `TestLiveSCRAM` | Live (client) | TestLiveSCRAM authenticates against a real TrueNAS box via SCRAM-SHA-512. |
| `TestLiveSCRAM_WrongCredentialsRejected` | Live (client) | TestLiveSCRAM_WrongCredentialsRejected verifies the REAL middleware rejects SCRAM exchanges built from a wrong secret and from a wrong username — the negative half of the protocol, judged by the server itself rather than any simulated one. |
| `TestBuildTLSConfig_BadCAFile` | Unit |  |
| `TestBuildTLSConfig_Default` | Unit |  |
| `TestBuildTLSConfig_Insecure` | Unit |  |
| `TestBuildTLSConfig_ValidCAFile` | Unit |  |
| `TestIsNotFound` | Unit |  |
| `TestIsRateLimited` | Unit | TestIsRateLimited is a regression test for a live acceptance failure: "auth.login_with_api_key: truenas API error (code 16): [EBUSY] Rate Limit Exceeded". |
| `TestIsTransient` | Unit |  |
| `TestNew_WSS_Rejection` | Unit | Ensure Client works with ws:// (not wss://) in tests |
| `TestScramConversation_BadServerFirstRejected` | Unit | TestScramConversation_BadServerFirstRejected verifies the client enforces the truenas_scram wire limits on the server-first message. |
| `TestScramConversation_BadServerSignatureRejected` | Unit | TestScramConversation_BadServerSignatureRejected verifies the client rejects a signature it cannot reproduce (mutual auth). |
| `TestScramConversation_ClientFirstWireForm` | Unit | TestScramConversation_ClientFirstWireForm pins the exact client-first message bytes: GS2 header, username:key_id identity, base64 nonce. |
| `TestScramConversation_NonceMismatchRejected` | Unit | TestScramConversation_NonceMismatchRejected verifies the client refuses a server nonce that does not extend the client nonce. |
| `TestSplitAPIKey` | Unit |  |
| `TestVersionAtLeast` | Unit |  |

## `internal/resources/acl_template`  (14)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccAclTemplate_basic` | Acceptance · Tier 1 | TestAccAclTemplate_basic creates a user-defined NFS4 ACL template (entry shape probed live via filesystem.acltemplate.create/get_instance), checks its attributes, updates the comment and the acl entries in place, imports it by its numeric id, and verifies destruction. |
| `TestAclDrifted_NullVsMinusOneNotDrift` | Unit | TestAclDrifted_NullVsMinusOneNotDrift verifies the two id sentinels don't trigger drift against each other (this is the exact null-vs-null-become- -1 quirk observed probing filesystem.acltemplate.create/get_instance live). |
| `TestAclDrifted_RealChangeIsDrift` | Unit | TestAclDrifted_RealChangeIsDrift verifies an actual permission change is still detected as drift. |
| `TestAclEntriesNormalized_StripsIDSentinels` | Unit | TestAclEntriesNormalized_StripsIDSentinels verifies both observed "no id" sentinels (null and -1, probed live) are dropped, and a real USER/GROUP id (e.g. |
| `TestAclTemplateDataSourceModel_MatchesSchema` | Unit | TestAclTemplateDataSourceModel_MatchesSchema verifies that every tfsdk tag on AclTemplateDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestAclTemplateSchema_ACLTypeOneOf` | Unit | TestAclTemplateSchema_ACLTypeOneOf verifies "acltype" is constrained to the two values filesystem.acltemplate.create accepts (probed live). |
| `TestAclTemplateSchema_BuiltinIsComputed` | Unit | TestAclTemplateSchema_BuiltinIsComputed verifies "builtin" is Computed-only: this resource never lets a user declare a template as builtin (the server assigns that). |
| `TestAclTemplateSchema_CommentOptionalComputed` | Unit | TestAclTemplateSchema_CommentOptionalComputed verifies "comment" carries the TrueNAS-side default ("") via Optional+Computed. |
| `TestAclTemplateSchema_IDIsComputed` | Unit | TestAclTemplateSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestAclTemplateSchema_RequiredFields` | Unit | TestAclTemplateSchema_RequiredFields verifies "name", "acltype", and "acl" are Required, matching filesystem.acltemplate.create's own "required" list (probed via core.get_methods). |
| `TestApiPayload_CommentOmittedWhenUnset` | Unit |  |
| `TestApiPayload_FullySet` | Unit |  |
| `TestResponseToDataSourceModel` | Unit |  |
| `TestResponseToModel` | Unit |  |

## `internal/resources/acme_dns_authenticator`  (12)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccAcmeDnsAuthenticator_basic` | Acceptance · Tier 1 | TestAccAcmeDnsAuthenticator_basic exercises the full Tier 1 contract for a cloudflare-variant ACME DNS authenticator. |
| `TestAcmeDnsAuthenticatorDataSourceModel_MatchesSchema` | Unit | TestAcmeDnsAuthenticatorDataSourceModel_MatchesSchema verifies that every tfsdk tag on AcmeDnsAuthenticatorDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestAttributesDrifted_APIAddedKeyIgnored` | Unit | TestAttributesDrifted_APIAddedKeyIgnored verifies attributesDrifted does not flag keys the API adds that weren't in the user's original state. |
| `TestAttributesDrifted_UserKeyChanged` | Unit | TestAttributesDrifted_UserKeyChanged verifies attributesDrifted detects a changed user-set key. |
| `TestAttributesMap_InvalidJSON` | Unit | TestAttributesMap_InvalidJSON verifies attributesMap rejects malformed JSON. |
| `TestAttributesMap_RequiresAuthenticatorKey` | Unit | TestAttributesMap_RequiresAuthenticatorKey verifies attributesMap rejects JSON missing the "authenticator" discriminator key. |
| `TestCreatePayload_Cloudflare` | Unit | TestCreatePayload_Cloudflare verifies createPayload against the shape probed live from a real acme.dns.authenticator.create call. |
| `TestResponseToModel_ProbedShape` | Unit | TestResponseToModel_ProbedShape verifies responseToModel against the exact shape observed from a live acme.dns.authenticator.create call (cloudflare variant, dummy api_token — credentials returned unmasked). |
| `TestSchema_AttributesSensitiveNotWriteOnly` | Unit | TestSchema_AttributesSensitiveNotWriteOnly verifies "attributes" is Required+Sensitive but NOT Computed/WriteOnly — see model.go's acmeDnsAuthenticatorAPI doc comment for the live probe evidence (credentials returned in cleartext by create/get_instance/query). |
| `TestSchema_IDComputedUseStateForUnknown` | Unit | TestSchema_IDComputedUseStateForUnknown verifies "id" is Computed-only. |
| `TestSchema_TopLevelRequiredFields` | Unit | TestSchema_TopLevelRequiredFields verifies "name" and "attributes" are Required, matching acme.dns.authenticator.create's own "required" list. |
| `TestUpdatePayload_SameShapeAsCreate` | Unit | TestUpdatePayload_SameShapeAsCreate verifies updatePayload builds the same {name, attributes} shape as createPayload — probed live: acme.dns.authenticator.update accepts a full replace, not a partial patch. |

## `internal/resources/alert_policy`  (15)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccAlertPolicy_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccAlertPolicy_setAndRestore drives the singleton truenas_alert_policy resource's "classes" field through a one-class override (UPSBatteryLow: WARNING/DAILY) layered on top of the box's existing classes, and back to the exact classes map read from the box before the test ran, then imports it. |
| `TestAlertPolicySchema_ClassesIsRequired` | Unit | TestAlertPolicySchema_ClassesIsRequired verifies that "classes" is a Required (not Sensitive) StringAttribute. |
| `TestAlertPolicySchema_IDIsComputed` | Unit | TestAlertPolicySchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute, since it's a fixed singleton value never supplied by the user. |
| `TestApplyAPIToModel_DifferentOverwritesState` | Unit | TestApplyAPIToModel_DifferentOverwritesState verifies that when the API's classes differ from state, the state string is overwritten with the API's JSON. |
| `TestApplyAPIToModel_EmptyAPIClasses` | Unit | TestApplyAPIToModel_EmptyAPIClasses verifies that an empty API classes object ("{}", the post-Delete/default state) round-trips correctly. |
| `TestApplyAPIToModel_EqualKeepsStateString` | Unit | TestApplyAPIToModel_EqualKeepsStateString verifies that when the state's classes JSON deep-equals the API's classes (even with different key order/whitespace in the JSON text), Read/Create/Update keep the state's original string rather than overwriting it with the API's canonical form. |
| `TestApplyAPIToModel_NullStatePopulatesFromAPI` | Unit | TestApplyAPIToModel_NullStatePopulatesFromAPI verifies the import path: when state's Classes is null (as it is right after ImportState sets only id), applyAPIToModel populates Classes directly from the API response. |
| `TestClassesEqual_DifferentKeys` | Unit | TestClassesEqual_DifferentKeys verifies that classesEqual reports drift when classes are added or removed (whole-object ownership, not a subset comparison). |
| `TestClassesEqual_DifferentValues` | Unit | TestClassesEqual_DifferentValues verifies that classesEqual reports drift when a class's value differs. |
| `TestClassesEqual_SameContentDifferentKeyOrder` | Unit | TestClassesEqual_SameContentDifferentKeyOrder verifies that classesEqual treats maps with identical content as equal, independent of the key order in the original JSON text used to build them (Go maps have no key order, so this really tests that DeepEqual is applied to parsed maps, not raw strings). |
| `TestClassesMap_InvalidJSON` | Unit | TestClassesMap_InvalidJSON verifies that invalid classes JSON produces a diagnostic error rather than a panic or silent failure. |
| `TestClassesMap_ValidJSON` | Unit | TestClassesMap_ValidJSON verifies that valid classes JSON (including the empty object) parses without error. |
| `TestDeletePayload` | Unit | TestDeletePayload verifies that Delete's payload builder always sends an empty classes object, resetting all classes to their TrueNAS defaults. |
| `TestUpdatePayload_InvalidJSON` | Unit | TestUpdatePayload_InvalidJSON verifies that updatePayload surfaces the same diagnostic as classesMap for invalid JSON, and returns a nil payload. |
| `TestUpdatePayload_Valid` | Unit | TestUpdatePayload_Valid verifies that updatePayload wraps the parsed classes map under the "classes" key. |

## `internal/resources/alert_service`  (16)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccAlertService_basic` | Acceptance · Tier 1 | TestAccAlertService_basic tests create, update, and import of a Mail alert service. |
| `TestAttributesDrifted_APIAddedKey` | Unit | TestAttributesDrifted_APIAddedKey verifies that a key present only in the API response (a server-added default) is NOT considered drift. |
| `TestAttributesDrifted_ChangedValue` | Unit | TestAttributesDrifted_ChangedValue verifies that a changed value for a user-set key is reported as drift. |
| `TestAttributesDrifted_Identical` | Unit | TestAttributesDrifted_Identical verifies that identical maps are not considered drifted. |
| `TestAttributesDrifted_MissingUserKey` | Unit | TestAttributesDrifted_MissingUserKey verifies that a user-set key absent from the API response is considered drift. |
| `TestAttributesMap_InvalidJSON` | Unit | TestAttributesMap_InvalidJSON verifies that attributesMap rejects malformed JSON with an error diagnostic. |
| `TestAttributesMap_MissingType` | Unit | TestAttributesMap_MissingType verifies that attributesMap rejects valid JSON that lacks the required "type" key. |
| `TestAttributesMap_Valid` | Unit | TestAttributesMap_Valid verifies that attributesMap parses valid JSON with a type key without error. |
| `TestCreatePayload_EnabledOmittedWhenUnset` | Unit | TestCreatePayload_EnabledOmittedWhenUnset verifies that enabled is omitted from the payload when it is null or unknown. |
| `TestCreatePayload_Keys` | Unit | TestCreatePayload_Keys verifies createPayload always includes name, level, and attributes, and includes enabled when it is known and non-null. |
| `TestSchema_AttributesSensitiveRequired` | Unit | TestSchema_AttributesSensitiveRequired verifies that attributes is Required and Sensitive. |
| `TestSchema_EnabledOptionalComputed` | Unit | TestSchema_EnabledOptionalComputed verifies that "enabled" is Optional + Computed with a UseStateForUnknown plan modifier. |
| `TestSchema_IDComputedUseStateForUnknown` | Unit | TestSchema_IDComputedUseStateForUnknown verifies that the "id" attribute is Computed with a UseStateForUnknown plan modifier. |
| `TestSchema_LevelRequired` | Unit | TestSchema_LevelRequired verifies that "level" is a Required StringAttribute. |
| `TestSchema_NameRequired` | Unit | TestSchema_NameRequired verifies that "name" is a Required StringAttribute. |
| `TestUpdatePayload_Keys` | Unit | TestUpdatePayload_Keys verifies updatePayload also includes name, level, and attributes always, with enabled guarded the same way. |

## `internal/resources/api_key`  (22)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccAPIKey_basic` | Acceptance · Tier 1 | TestAccAPIKey_basic creates an API key for the acceptance-test user, proves the created key actually authenticates against the real server (opening a second client.Client and calling system.version_short through it — the SCRAM path on TrueNAS 26.0+), renames the key in place, imports it by numeric id, and verifies destruction. |
| `TestAPIKeyDataSourceModel_MatchesSchema` | Unit | TestAPIKeyDataSourceModel_MatchesSchema verifies that every tfsdk tag on APIKeyDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestAPIKeySchema_ComputedReadbacks` | Unit | TestAPIKeySchema_ComputedReadbacks verifies created_at, local, and revoked are Computed-only (server-reported, never user-set). |
| `TestAPIKeySchema_ExpiresAtOptionalComputed` | Unit | TestAPIKeySchema_ExpiresAtOptionalComputed verifies "expires_at" is Optional+Computed (nullable, user-settable, but also always populated from the API). |
| `TestAPIKeySchema_IDIsComputed` | Unit | TestAPIKeySchema_IDIsComputed verifies "id" is Computed-only. |
| `TestAPIKeySchema_KeyIsSensitiveAndComputed` | Unit | TestAPIKeySchema_KeyIsSensitiveAndComputed verifies "key" is both Sensitive and Computed-only (never Required/Optional — TrueNAS is the sole source of the plaintext value). |
| `TestAPIKeySchema_NameRequired` | Unit | TestAPIKeySchema_NameRequired verifies "name" is Required. |
| `TestAPIKeySchema_UsernameRequiresReplace` | Unit | TestAPIKeySchema_UsernameRequiresReplace verifies "username" is Required and forces replacement on change (api_key.update rejects changing it). |
| `TestCreatePayload_IncludesUsername` | Unit | TestCreatePayload_IncludesUsername verifies username is present in the create payload (Required, api_key.create-only field). |
| `TestCreatePayload_InvalidExpiresAt` | Unit | TestCreatePayload_InvalidExpiresAt verifies createPayload surfaces the diagnostic error from expiresAtToPayload instead of building a partial payload. |
| `TestDecodeDateField_Invalid` | Unit |  |
| `TestDecodeDateField_Null` | Unit | --- decodeDateField --- |
| `TestDecodeDateField_Value` | Unit |  |
| `TestExpiresAtToPayload_InvalidFormat` | Unit | TestExpiresAtToPayload_InvalidFormat verifies a non-RFC3339 string produces a diagnostic error rather than a fallback value. |
| `TestExpiresAtToPayload_Null` | Unit | TestExpiresAtToPayload_Null verifies an unset/null model value encodes to a nil payload entry (JSON null on the wire), matching "no expiration". |
| `TestExpiresAtToPayload_Value` | Unit | TestExpiresAtToPayload_Value verifies a valid RFC3339 string encodes to the {"$date": <ms>} shape api_key.create/update accept (probed live: an ISO datetime string is rejected outright, only this extended-JSON shape works). |
| `TestResponseToDataSourceModel_Basic` | Unit | TestResponseToDataSourceModel_Basic verifies the datasource mapping (which has no Key field to preserve or omit). |
| `TestResponseToModel_ExpiresAtValue` | Unit | TestResponseToModel_ExpiresAtValue verifies a set expires_at round-trips through decodeDateField into the RFC3339 UTC string form. |
| `TestResponseToModel_MissingCreatedAt` | Unit | TestResponseToModel_MissingCreatedAt verifies a null created_at (which the live API never actually returns — it is always required — but which would indicate a wire-shape regression) surfaces as an error rather than silently producing a zero-value timestamp. |
| `TestResponseToModel_PreservesKeyOnRead` | Unit | TestResponseToModel_PreservesKeyOnRead verifies responseToModel never touches m.Key, regardless of what is already there — the central invariant this resource depends on to avoid ever nulling out a previously-captured plaintext key on Read/Update. |
| `TestUpdatePayload_AlwaysSendsExpiresAt` | Unit | TestUpdatePayload_AlwaysSendsExpiresAt verifies expires_at is present in the update payload even when null — unlike api_key.create, an *omitted* expires_at on api_key.update means "leave unchanged" (probed live), so clearing a previously-set expiration requires sending an explicit null on every update, not omitting the field. |
| `TestUpdatePayload_OmitsUsername` | Unit | TestUpdatePayload_OmitsUsername verifies username is never present in the update payload: api_key.update rejects it outright as an unrecognized field (probed live: "Extra inputs are not permitted"), matching the schema's RequiresReplace plan modifier on username. |

## `internal/resources/app`  (14)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccApp_basic` | Acceptance · Apps gate | TestAccApp_basic installs the "syncthing" catalog app with default values, verifies computed attributes are populated, stops it via the "running" attribute, and finally destroys it. |
| `TestAccApp_datasource` | Acceptance · Apps gate | TestAccApp_datasource verifies that the truenas_app datasource can look up an app created by the resource in the same config. |
| `TestAppSchema_IDIsString` | Unit | TestAppSchema_IDIsString verifies that "id" is a string attribute (the app name is used as the Terraform ID, not a numeric ID). |
| `TestAppSchema_NameIsRequiresReplace` | Unit | TestAppSchema_NameIsRequiresReplace verifies that "name" carries the RequiresReplace plan modifier, forcing resource recreation if the app name changes. |
| `TestAppSchema_StateHasNoUseStateForUnknown` | Unit | TestAppSchema_StateHasNoUseStateForUnknown verifies that "state", "human_version" and "upgrade_available" are Computed WITHOUT UseStateForUnknown, since they legitimately change server-side between applies. |
| `TestCreatePayload_IncludesSetFields` | Unit | TestCreatePayload_IncludesSetFields verifies that createPayload includes fields that are explicitly set in the plan. |
| `TestCreatePayload_InvalidValuesJSONProducesDiagnostic` | Unit | TestCreatePayload_InvalidValuesJSONProducesDiagnostic verifies that createPayload surfaces the values-JSON error as a diagnostic rather than silently dropping it. |
| `TestCreatePayload_OmitsUnsetOptionalFields` | Unit | TestCreatePayload_OmitsUnsetOptionalFields verifies that createPayload omits catalog_app and values when they are unset (null) in the plan. |
| `TestNeedsUpgrade` | Unit | TestNeedsUpgrade verifies that needsUpgrade correctly detects a plan-vs-state version change, which drives the app.upgrade call in Update. |
| `TestResponseToDatasourceModel_AllFields` | Unit | TestResponseToDatasourceModel_AllFields verifies field mapping for the datasource model. |
| `TestResponseToModel_IDIsAppName` | Unit | TestResponseToModel_IDIsAppName verifies that the Terraform ID is set to the app name string. |
| `TestResponseToModel_NeverTouchesWriteOnlyFields` | Unit | TestResponseToModel_NeverTouchesWriteOnlyFields verifies that responseToModel does not modify Values, ComposeYAML, or CatalogApp, since the API never echoes these back (write-only pattern). |
| `TestResponseToModel_Running` | Unit | TestResponseToModel_Running verifies that State=="RUNNING" maps to Running=true and any other state maps to Running=false. |
| `TestValuesMap_InvalidJSON` | Unit | TestValuesMap_InvalidJSON verifies that invalid JSON in the "values" field produces an error diagnostic instead of panicking. |

## `internal/resources/app_registry`  (14)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccAppRegistry_basic` | Acceptance · conditional skip | TestAccAppRegistry_basic is intentionally skipped unconditionally. |
| `TestApiPayload_DescriptionExplicitNull` | Unit | TestApiPayload_DescriptionExplicitNull verifies that a null Description is sent as an explicit "description": nil in the payload (not omitted) — required so app.registry.update (a partial update where omitted keys mean "no change") can clear a previously-set description back to null. |
| `TestApiPayload_FullySet` | Unit | TestApiPayload_FullySet verifies apiPayload includes name, username, password (from the passed-in config value, not m.Password), description, and uri when all are set. |
| `TestApiPayload_URIOmittedWhenUnknown` | Unit | TestApiPayload_URIOmittedWhenUnknown verifies that "uri" is omitted from the payload when unknown (the create-time case when the user doesn't set it: Optional+Computed with no prior state produces an Unknown planned value), letting the TrueNAS-side default ("https://index.docker.io/v1/") apply. |
| `TestAppRegistryDataSourceModel_MatchesSchema` | Unit | TestAppRegistryDataSourceModel_MatchesSchema verifies that every tfsdk tag on AppRegistryDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestAppRegistryDataSourceModel_NoPasswordField` | Unit | TestAppRegistryDataSourceModel_NoPasswordField verifies (via reflection) that AppRegistryDataSourceModel has no "password" field, so registry credentials can never be exposed through the datasource. |
| `TestResponseToDataSourceModel_ProbedShape` | Unit | TestResponseToDataSourceModel_ProbedShape mirrors TestResponseToModel_ProbedShape for the datasource model, and verifies AppRegistryDataSourceModel has no "password" field at all (via reflection), mirroring iscsi_auth's datasource-security test. |
| `TestResponseToModel_DescriptionNull` | Unit | TestResponseToModel_DescriptionNull verifies a nil api.Description maps to a null Description, and that Password stays null (not coerced to "") when the model started null — mirrors iscsi_auth's equivalent test for its write-only secrets. |
| `TestResponseToModel_ProbedShape` | Unit | TestResponseToModel_ProbedShape verifies responseToModel against the field shape probed live from app.registry.create/get_instance/query/update (core.get_methods, both TrueNAS 25.10 and 26.0): id, name, description (nullable), uri, username. |
| `TestSchema_DescriptionOptionalNotComputed` | Unit | TestSchema_DescriptionOptionalNotComputed verifies "description" is Optional (nullable) and NOT Computed: app.registry.create's server-side default is null (not a computed non-null value), so no UseStateForUnknown plan modifier is needed and omitting it never causes drift. |
| `TestSchema_IDComputedUseStateForUnknown` | Unit | TestSchema_IDComputedUseStateForUnknown verifies that the "id" attribute is Computed-only. |
| `TestSchema_PasswordRequiredSensitiveWriteOnly` | Unit | TestSchema_PasswordRequiredSensitiveWriteOnly verifies "password" is Required, Sensitive, WriteOnly, and NOT Computed: app.registry.create requires it (probed live on both releases), the API documents it as "masked for security", and no live read-back evidence exists to persist it in state (see model.go's doc comment for the full decisive-probe writeup on why no entry could be created to observe a read-back value). |
| `TestSchema_TopLevelRequiredFields` | Unit | TestSchema_TopLevelRequiredFields verifies "name" and "username" are Required, matching app.registry.create's own "required" list (probed live on both TrueNAS 25.10 and 26.0). |
| `TestSchema_URIOptionalComputed` | Unit | TestSchema_URIOptionalComputed verifies "uri" is Optional+Computed: probed live, app.registry.create defaults uri server-side to "https://index.docker.io/v1/" when omitted, so it needs UseStateForUnknown to avoid perpetual diffs. |

## `internal/resources/audit_config`  (12)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccAuditConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccAuditConfigDataSource_basic reads the current TrueNAS audit configuration through the truenas_audit_config datasource only. |
| `TestAccAuditConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccAuditConfig_setAndRestore drives the singleton truenas_audit_config resource's "quota_fill_warning" field (range 5-80) through a different valid value and back to the value read from the box before the test ran, then imports it. |
| `TestAuditConfigDataSourceModel_MatchesSchema` | Unit | TestAuditConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on AuditConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestAuditConfigSchema_IDIsComputed` | Unit | TestAuditConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestAuditConfigSchema_ReadOnlyFieldsAreComputedOnly` | Unit | TestAuditConfigSchema_ReadOnlyFieldsAreComputedOnly verifies that remote_logging_enabled, space, and enabled_services are Computed-only: audit.update does not accept any of them. |
| `TestAuditConfigSchema_SettableFieldsAreOptionalComputed` | Unit | TestAuditConfigSchema_SettableFieldsAreOptionalComputed verifies that retention, reservation, quota, quota_fill_warning, and quota_fill_critical are all Optional+Computed with UseStateForUnknown plan modifiers, matching the fields audit.update actually accepts. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable. |
| `TestResponseToModel` | Unit | TestResponseToModel verifies responseToModel against the shape probed from a live audit.config call. |
| `TestResponseToModel_NilServiceListsBecomeEmptyLists` | Unit | TestResponseToModel_NilServiceListsBecomeEmptyLists verifies that nil []string slices from the API map to empty (non-null) lists, matching the nil-guard convention used elsewhere for API-returned lists. |
| `TestUpdatePayload_AllFieldsSet` | Unit | TestUpdatePayload_AllFieldsSet verifies that updatePayload includes every field with the exact keys observed in the audit.update probe. |
| `TestUpdatePayload_RetentionOnly` | Unit | TestUpdatePayload_RetentionOnly verifies the shape updatePayload actually sees on the real Create/Update path when the caller passes a model built from req.Config: an HCL config that sets only "retention" leaves "reservation", "quota", "quota_fill_warning", and "quota_fill_critical" null in config — NOT Unknown (Unknown never occurs in req.Config; Terraform resolves config to either a concrete value or null before the provider ever sees it). |
| `TestUpdatePayload_UnsetOptionalsOmitted` | Unit | TestUpdatePayload_UnsetOptionalsOmitted verifies that every field is omitted when null/unknown, so the current TrueNAS-side value is left unchanged rather than overwritten with a zero value. |

## `internal/resources/boot_environment`  (18)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccBootEnvironment_basic` | Acceptance · Tier 1 | TestAccBootEnvironment_basic clones the currently active boot environment, exercises the keep attribute, imports, and finally destroys only the clone. |
| `TestAccBootEnvironment_datasource` | Acceptance · Tier 1 | TestAccBootEnvironment_datasource verifies that the truenas_boot_environment datasource can look up a boot environment cloned by the resource in the same config. |
| `TestClonePayloadShape` | Unit | TestClonePayloadShape verifies the exact shape of the clone() call args: {"id": source, "target": name}. |
| `TestDeleteGuard_ActiveOrActivatedBlocksDestroy` | Unit | TestDeleteGuard_ActiveOrActivatedBlocksDestroy verifies the pure decision logic behind Delete: a BE that is active or activated must never be destroyed. |
| `TestKeepPayloadShape` | Unit | TestKeepPayloadShape verifies the exact shape of the keep() call args: {"id": name, "value": v}. |
| `TestPlanActivationChange_FalseToTrue` | Unit | TestPlanActivationChange_FalseToTrue verifies that activating an inactive BE (state=false -> plan=true) requests an activate call. |
| `TestPlanActivationChange_NoChange` | Unit |  |
| `TestPlanActivationChange_TrueToFalse` | Unit | TestPlanActivationChange_TrueToFalse verifies that flipping an activated BE back to inactive is flagged as unsupported: the API has no "deactivate" operation, only activate-a-different-BE. |
| `TestResponseToModel_AllFields` | Unit | --- responseToModel -------------------------------------------------- |
| `TestResponseToModel_DoesNotTouchSource` | Unit | TestResponseToModel_DoesNotTouchSource verifies that responseToModel never overwrites Source: the API never returns a clone-origin field, so whatever value already lives in state/plan must survive untouched. |
| `TestSchema_ActivatedIsOptionalComputed` | Unit |  |
| `TestSchema_ActiveAndUsedBytesHaveNoPlanModifiers` | Unit | TestSchema_ActiveAndUsedBytesHaveNoPlanModifiers verifies that "active" and "used_bytes" are Computed-only with NO plan modifiers, because they are server-mutable (e.g. |
| `TestSchema_DatasetHasUseStateForUnknown` | Unit |  |
| `TestSchema_IDIsComputedString` | Unit | --- Schema shape --------------------------------------------------------- |
| `TestSchema_KeepIsOptionalComputed` | Unit |  |
| `TestSchema_NameIsRequiresReplace` | Unit |  |
| `TestSchema_SourceIsRequiresReplace` | Unit |  |
| `TestUpdate_ActivatedTrueToFalseProducesError` | Unit | TestUpdate_ActivatedTrueToFalseProducesError exercises the Update method end-to-end (without a live API) to confirm that requesting activated: true -> false surfaces as an error diagnostic rather than silently succeeding or panicking. |

## `internal/resources/catalog_config`  (12)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccCatalogConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccCatalogConfigDataSource_basic reads the current TrueNAS app catalog configuration through the truenas_catalog_config datasource only. |
| `TestAccCatalogConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccCatalogConfig_setAndRestore drives the singleton truenas_catalog_config resource's "preferred_trains" field through one train removed (if "community" is currently preferred — the observed state on both probed boxes) or added (if it is absent), then back to the value read from the box before the test ran, then imports it. |
| `TestCatalogConfigDataSourceModel_MatchesSchema` | Unit | TestCatalogConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on CatalogConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestCatalogConfigSchema_IDIsComputed` | Unit | TestCatalogConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestCatalogConfigSchema_PreferredTrainsIsOptionalComputed` | Unit | TestCatalogConfigSchema_PreferredTrainsIsOptionalComputed verifies preferred_trains — the only field catalog.update actually accepts (per the probe) — is Optional+Computed with a plan modifier. |
| `TestCatalogConfigSchema_ReadOnlyFieldsAreComputedOnly` | Unit | TestCatalogConfigSchema_ReadOnlyFieldsAreComputedOnly verifies label and location are Computed-only: catalog.update does not accept either (probed live — accepts schema is {preferred_trains} only). |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies Delete's diagnostic builder returns exactly one warning and never touches a client. |
| `TestResponseToDataSourceModel` | Unit | TestResponseToDataSourceModel verifies responseToDataSourceModel maps both catalog.config's preferred_trains and the separately-fetched catalog.trains list correctly, including catalog.trains coming back empty (probed live on a TrueNAS 25.10 box with Docker/apps unconfigured). |
| `TestResponseToModel` | Unit | TestResponseToModel verifies responseToModel against the shape probed from a live catalog.config call (identical on TrueNAS 25.10 and 26.0). |
| `TestResponseToModel_NilPreferredTrainsBecomesEmptyList` | Unit | TestResponseToModel_NilPreferredTrainsBecomesEmptyList verifies a nil []string from the API maps to an empty (non-null) list, matching the nil-guard convention established by audit_config's stringListOrEmpty. |
| `TestUpdatePayload_PreferredTrainsSet` | Unit | TestUpdatePayload_PreferredTrainsSet verifies updatePayload includes preferred_trains with the exact key catalog.update accepts (probed live — see task-3-report.md). |
| `TestUpdatePayload_UnsetOptionalOmitted` | Unit | TestUpdatePayload_UnsetOptionalOmitted verifies preferred_trains is omitted when null/unknown, so the current TrueNAS-side value is left unchanged rather than overwritten with an explicit empty list. |

## `internal/resources/certificate`  (27)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccCertificate_acmeIssuance` | Acceptance · Tier 1 | TestAccCertificate_acmeIssuance drives a full, live ACME certificate order end to end through Terraform against a real ACME CA (a Pebble test server), using the DNS-01 "shell" authenticator wired to pebble-challtestsrv. |
| `TestAccCertificate_csr` | Acceptance · Tier 1 | TestAccCertificate_csr exercises the CERTIFICATE_CREATE_CSR path: TrueNAS generates a new RSA 2048 key pair and CSR on-box. |
| `TestAccCertificate_imported` | Acceptance · Tier 1 | TestAccCertificate_imported exercises the full Tier 1 contract for create_type=CERTIFICATE_CREATE_IMPORTED: import a self-signed RSA 2048 cert+key generated in-test, verify the read-back fields, rename in place (the one field certificate.update accepts besides renew_days/ add_to_trusted_store, confirmed live), import by id, and verify destruction via a live certificate.query. |
| `TestAccCertificate_importedCSR` | Acceptance · Tier 1 | TestAccCertificate_importedCSR exercises the full Tier 1 contract for create_type=CERTIFICATE_CREATE_IMPORTED_CSR: import an externally-generated RSA 2048 CSR + its private key (both generated in-test, unlike CERTIFICATE_CREATE_CSR where TrueNAS generates the key pair on-box), verify the read-back fields, rename in place, import by id, and verify destruction via a live certificate.query. |
| `TestCertificateDataSourceModel_MatchesSchema` | Unit | TestCertificateDataSourceModel_MatchesSchema verifies that every tfsdk tag on CertificateDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestCreatePayload_Imported` | Unit | TestCreatePayload_Imported verifies createPayload includes name, create_type, certificate, and privatekey for the IMPORTED path, and omits unset optionals. |
| `TestPreflight_AcmeRequiresAllFour` | Unit | TestPreflight_AcmeRequiresAllFour verifies ACME requires acme_directory_uri, csr_id, tos (true), and dns_mapping (non-empty), per the task brief. |
| `TestPreflight_AcmeSatisfied` | Unit | TestPreflight_AcmeSatisfied verifies a fully-populated ACME model passes preflight cleanly. |
| `TestPreflight_CSRRequiresKeyLengthForRSA` | Unit | TestPreflight_CSRRequiresKeyLengthForRSA verifies CSR requires "key_length" when key_type is RSA (the default) — probed live: TrueNAS rejects an RSA CSR with no key_length ("RSA-based keys require an entry in this field"). |
| `TestPreflight_CSRRequiresSan` | Unit | TestPreflight_CSRRequiresSan verifies CSR requires a non-empty "san" — probed live: omitting it fails "List should have at least 1 item", even with "common" set. |
| `TestPreflight_ImportedCSRRequiresCSRAndKey` | Unit | TestPreflight_ImportedCSRRequiresCSRAndKey mirrors TestPreflight_ImportedRequiresCertAndKey for IMPORTED_CSR. |
| `TestPreflight_ImportedRequiresCertAndKey` | Unit | TestPreflight_ImportedRequiresCertAndKey verifies IMPORTED requires both "certificate" and "privatekey" (probed live: a bare CERTIFICATE_CREATE_IMPORTED payload with neither field set fails schema validation for both). |
| `TestResponseToModel_CSRTypeNullFields` | Unit | TestResponseToModel_CSRTypeNullFields verifies that CSR-type responses (certificate=nil, many parsed fields nil per live probe) map to null rather than panicking or zero-valuing. |
| `TestResponseToModel_ProbedShape` | Unit | TestResponseToModel_ProbedShape verifies responseToModel against the exact shape observed from a live certificate.create (IMPORTED)/ get_instance call on TrueNAS 25.10, including the masked-then- re-read privatekey and pointer-nullable parsed fields. |
| `TestSanListValue_StripsDNSPrefix` | Unit | TestSanListValue_StripsDNSPrefix verifies sanListValue strips the "DNS:" prefix the API adds to every "san" entry it returns — probed live: certificate.create accepts a plain hostname but certificate.query/ get_instance echo it back prefixed. |
| `TestSchema_CreateTypeRequiresReplace` | Unit | TestSchema_CreateTypeRequiresReplace verifies "create_type" carries RequiresReplace, since certificate.update never accepts it (probed live). |
| `TestSchema_IDComputedUseStateForUnknown` | Unit | TestSchema_IDComputedUseStateForUnknown verifies "id" is Computed-only. |
| `TestSchema_PassphraseWriteOnlyByConvention` | Unit | TestSchema_PassphraseWriteOnlyByConvention verifies "passphrase" is Sensitive and Optional but deliberately NOT Computed, since the API never echoes it back (probed live: absent from every create/update/ get_instance/query response). |
| `TestSchema_PrivatekeySensitiveNotWriteOnly` | Unit | TestSchema_PrivatekeySensitiveNotWriteOnly verifies "privatekey" is Sensitive+Computed (read back on refresh) rather than WriteOnly — see model.go's certificateAPI doc comment for the live probe evidence (certificate.get_instance returns it intact; only the create/update job result masks it). |
| `TestSchema_TopLevelRequiredFields` | Unit | TestSchema_TopLevelRequiredFields verifies "name" and "create_type" are Required, matching certificate.create's own "required" list. |
| `TestSchema_UpdatableFieldsNotRequiresReplace` | Unit | TestSchema_UpdatableFieldsNotRequiresReplace verifies the three fields certificate.update actually accepts (name, renew_days, add_to_trusted_store — probed live) do NOT carry RequiresReplace, while spot-checking that an immutable field (certificate) does. |
| `TestUpdatePayload_CSROmitsAddToTrustedStoreAndRenewDays` | Unit | TestUpdatePayload_CSROmitsAddToTrustedStoreAndRenewDays verifies that for CERTIFICATE_CREATE_CSR (an unsigned CSR entry), updatePayload omits both "add_to_trusted_store" (rejected live: "A CSR cannot be added to the system's trusted store") and "renew_days" — only "name" survives. |
| `TestUpdatePayload_ImportedOmitsRenewDays` | Unit | TestUpdatePayload_ImportedOmitsRenewDays verifies that for CERTIFICATE_CREATE_IMPORTED, updatePayload includes "add_to_trusted_store" but omits "renew_days" entirely — sending it would fail live with "[EINVAL] certificate_update.renew_days: Certificate renewal days is only supported for ACME certificates", even though renew_days is Optional+ Computed and therefore always known once a prior apply has populated it. |
| `TestUpdatePayload_OnlyUpdatableFields` | Unit | TestUpdatePayload_OnlyUpdatableFields verifies updatePayload includes name/renew_days/add_to_trusted_store — the top-level set certificate.update's schema accepts (probed live) — but ALSO gates "add_to_trusted_store" and "renew_days" by create_type, since certificate.update's own service-level validation rejects both outside specific create_types (probed live; see updatePayload's doc comment): "add_to_trusted_store" only for CERTIFICATE_CREATE_IMPORTED/_ACME (never for an unsigned CSR entry), "renew_days" only for CERTIFICATE_CREATE_ACME. |
| `TestValidateRenewDays_AllowedForAcme` | Unit | TestValidateRenewDays_AllowedForAcme verifies ACME may set renew_days freely. |
| `TestValidateRenewDays_AllowedWhenUnsetInConfig` | Unit | TestValidateRenewDays_AllowedWhenUnsetInConfig verifies a non-ACME create_type with renew_days left null in Config (the caller never wrote it in HCL) passes cleanly, even though the same field could be a known, non-null value in Plan once a prior apply has populated it. |
| `TestValidateRenewDays_RejectedForNonAcmeConfig` | Unit | TestValidateRenewDays_RejectedForNonAcmeConfig verifies renew_days is rejected client-side for any non-ACME create_type when explicitly set in Config — probed live: certificate.update rejects it outright ("Certificate renewal days is only supported for ACME certificates") and certificate.create silently ignores/overrides it, either of which would otherwise surface as a confusing provider-inconsistency error rather than a clear preflight one. |

## `internal/resources/cloud_backup`  (22)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccCloudBackup_basic` | Acceptance · conditional skip | TestAccCloudBackup_basic is intentionally skipped unconditionally. |
| `TestApiPayload_OptionalFieldsIncludedWhenSet` | Unit | TestApiPayload_OptionalFieldsIncludedWhenSet verifies every Optional field is included in the payload once set, alongside the schedule sub-object. |
| `TestApiPayload_RequiredFieldsOnly` | Unit | TestApiPayload_RequiredFieldsOnly verifies that with every Optional field null/unknown, the payload contains exactly the five Required keys — path, credentials, attributes, password, keep_last — matching cloud_backup.create's own "required" list (probed via core.get_methods), so TrueNAS-side defaults take effect for everything else. |
| `TestAttributesDrifted_APIAddedKeyIgnored` | Unit | TestAttributesDrifted_APIAddedKeyIgnored verifies attributesDrifted does NOT report drift when the API merely adds default keys the user never set. |
| `TestAttributesDrifted_UserKeyChanged` | Unit | TestAttributesDrifted_UserKeyChanged verifies attributesDrifted reports drift when a user-set key's value differs between state and the API. |
| `TestCloudBackupDataSourceModel_MatchesSchema` | Unit | TestCloudBackupDataSourceModel_MatchesSchema verifies that every tfsdk tag on CloudBackupDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestCloudBackupSchema_AbsolutePathsRequiresReplace` | Unit | TestCloudBackupSchema_AbsolutePathsRequiresReplace verifies "absolute_paths" forces replacement: cloud_backup.update's accepted fields (probed via core.get_methods) exclude it — it's create-only. |
| `TestCloudBackupSchema_IDIsComputed` | Unit | TestCloudBackupSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestCloudBackupSchema_IncludeExcludeAreLists` | Unit | TestCloudBackupSchema_IncludeExcludeAreLists verifies "include"/"exclude" are Optional+Computed lists of strings. |
| `TestCloudBackupSchema_OptionalComputedFields` | Unit | TestCloudBackupSchema_OptionalComputedFields verifies the fields carrying TrueNAS-side defaults are Optional+Computed. |
| `TestCloudBackupSchema_PasswordIsSensitiveNotWriteOnly` | Unit | TestCloudBackupSchema_PasswordIsSensitiveNotWriteOnly verifies "password" is marked Sensitive but NOT WriteOnly: source-verified (middleware's CloudBackupEntry.password is a pydantic Secret[NonEmptyString], read back unmasked to a FULL_ADMIN/CLOUD_BACKUP_WRITE-scoped session — see schema.go's description), so it is modeled like keychain_ssh_keypair's private_key rather than truenas_user's WriteOnly password. |
| `TestCloudBackupSchema_RequiredFields` | Unit | TestCloudBackupSchema_RequiredFields verifies "path", "credentials", "attributes", "password", and "keep_last" are Required, matching cloud_backup.create's own "required" list (probed via core.get_methods). |
| `TestCloudBackupSchema_ScheduleShape` | Unit | TestCloudBackupSchema_ScheduleShape verifies the nested "schedule" attribute is Optional+Computed at the top level with Required string sub-fields, matching rsync_task's/cloudsync's convention. |
| `TestCloudBackupSchema_TransferSettingValidator` | Unit | TestCloudBackupSchema_TransferSettingValidator verifies "transfer_setting" carries an enum validator matching the choices returned by cloud_backup.transfer_setting_choices (probed live). |
| `TestDecodeCredentialsID_BareInteger` | Unit | TestDecodeCredentialsID_BareInteger verifies defensive support for a bare integer, in case a future API version returns one directly. |
| `TestDecodeCredentialsID_EmbeddedObject` | Unit | TestDecodeCredentialsID_EmbeddedObject verifies the shape actually probed live from cloudsync.credentials.create/cloud_backup responses: an embedded CloudCredentialEntry object. |
| `TestDecodeCredentialsID_Invalid` | Unit | TestDecodeCredentialsID_Invalid verifies an undecodable shape returns an error rather than silently defaulting to 0. |
| `TestDecodeCredentialsID_Null` | Unit | TestDecodeCredentialsID_Null verifies a JSON null is rejected: credentials is Required on cloud_backup.create, so a null in a response indicates a malformed/unexpected API shape, not a legitimate "unset" state. |
| `TestResponseToDataSourceModel_FullShape` | Unit | TestResponseToDataSourceModel_FullShape mirrors TestResponseToModel_FullShape for the datasource model, and additionally verifies Attributes is populated as canonical JSON (the datasource has no plan to preserve write-what-you-said against, unlike the resource). |
| `TestResponseToModel_FullShape` | Unit | TestResponseToModel_FullShape verifies responseToModel against the field shape documented by core.get_methods (cloud_backup.create/get_instance returns, live-probed on TrueNAS 25.10) and middlewared's api/v25_10_2/cloud_backup.py CloudBackupEntry model: credentials embedded, nullable cache_path/rate_limit, password returned verbatim. |
| `TestResponseToModel_NullableFieldsNull` | Unit | TestResponseToModel_NullableFieldsNull verifies cache_path and rate_limit decode to true Terraform null (not zero values) when the API returns null, matching the probed nullable shape. |
| `TestUpdatePayload_ExcludesAbsolutePaths` | Unit | TestUpdatePayload_ExcludesAbsolutePaths verifies updatePayload strips "absolute_paths" even when set on the model: cloud_backup.update's accepted fields (probed via core.get_methods, mirrored by middlewared's CloudBackupUpdate model) exclude it — it's create-only. |

## `internal/resources/cloudsync`  (23)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccCloudSync_basic` | Acceptance · conditional skip | TestAccCloudSync_basic tests create, update, and import of a cloud sync task. |
| `TestApiPayload_IncludeExcludeNil` | Unit | TestApiPayload_IncludeExcludeNil verifies that null/unknown include and exclude lists produce empty slices, never nil, in the payload. |
| `TestApiPayload_IncludesScheduleMap` | Unit | TestApiPayload_IncludesScheduleMap verifies apiPayload produces a schedule map with all 5 required keys and correct values. |
| `TestApiPayload_InvalidAttributesJSON` | Unit | TestApiPayload_InvalidAttributesJSON verifies that malformed attributes JSON produces an error diagnostic and a nil payload. |
| `TestApiPayload_OmitsUnsetOptionals` | Unit | TestApiPayload_OmitsUnsetOptionals verifies that enabled, snapshot, pre_script, and post_script are omitted from the payload when null/unknown (the plan-modifier state for unset Optional+Computed attributes), rather than being sent as their Go zero values (false/""). |
| `TestAttributesDrifted_APIAddedKey` | Unit |  |
| `TestAttributesDrifted_ChangedValue` | Unit |  |
| `TestAttributesDrifted_Identical` | Unit | --- attributes drift tests (copy pattern from vm_device) --- |
| `TestAttributesMap_InvalidJSON` | Unit |  |
| `TestCloudSyncSchema_AttributesRequired` | Unit | --- schema tests --- |
| `TestCloudSyncSchema_CredentialsRequired` | Unit |  |
| `TestCloudSyncSchema_DescriptionRequired` | Unit |  |
| `TestCloudSyncSchema_EnabledOptionalComputed` | Unit |  |
| `TestCloudSyncSchema_IncludeExcludeOptionalComputed` | Unit |  |
| `TestCloudSyncSchema_ScheduleNestedRequired` | Unit |  |
| `TestDecodeCredentialsID_BareInt` | Unit | TestDecodeCredentialsID_BareInt verifies decoding when credentials is a bare integer: 5. |
| `TestDecodeCredentialsID_Empty` | Unit | TestDecodeCredentialsID_Empty verifies that an empty (zero-length) raw message is also treated as an error, not decoded to 0. |
| `TestDecodeCredentialsID_Invalid` | Unit | TestDecodeCredentialsID_Invalid verifies decoding fails cleanly for unrecognized shapes. |
| `TestDecodeCredentialsID_Null` | Unit | TestDecodeCredentialsID_Null verifies that a JSON null credentials field returns an explicit error rather than silently decoding to 0. |
| `TestDecodeCredentialsID_Object` | Unit | TestDecodeCredentialsID_Object verifies decoding when credentials is an embedded object: {"id": 5, ...}. |
| `TestResponseToModel_CredentialsBareInt` | Unit | TestResponseToModel_CredentialsBareInt verifies responseToModel decodes a bare integer credentials field correctly. |
| `TestResponseToModel_CredentialsNull` | Unit | TestResponseToModel_CredentialsNull verifies that responseToModel surfaces an error diagnostic (rather than silently defaulting Credentials to 0) when the API returns a null credentials field. |
| `TestResponseToModel_CredentialsObject` | Unit | TestResponseToModel_CredentialsObject verifies responseToModel decodes an embedded credentials object correctly. |

## `internal/resources/cloudsync_credentials`  (15)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccCloudsyncCredentials_basic` | Acceptance · Tier 1 | TestAccCloudsyncCredentials_basic tests create, update, and import of cloud sync credentials. |
| `TestApiProviderJSON_MergesProviderAndAttributes` | Unit | TestApiProviderJSON_MergesProviderAndAttributes verifies apiProviderJSON serializes the wire provider object back into a provider_config-shaped JSON string. |
| `TestCreatePayload_WireKeyIsProvider` | Unit | TestCreatePayload_WireKeyIsProvider verifies createPayload sends provider_config as the "provider" wire object ({"type": ..., ...settings}, the /api/current shape) and includes name. |
| `TestCredentialsAPI_DecodesWireFormat` | Unit | TestCredentialsAPI_DecodesWireFormat verifies credentialsAPI decodes the /api/current response shape where "provider" is an object carrying the type discriminator and settings inline. |
| `TestProviderDrifted_APIAddedKey` | Unit | TestProviderDrifted_APIAddedKey verifies that a key present only in the API response (a server-added default) is NOT considered drift. |
| `TestProviderDrifted_ChangedValue` | Unit | TestProviderDrifted_ChangedValue verifies that a changed value for a user-set key is reported as drift. |
| `TestProviderDrifted_Identical` | Unit | TestProviderDrifted_Identical verifies that identical maps are not considered drifted. |
| `TestProviderDrifted_MissingUserKey` | Unit | TestProviderDrifted_MissingUserKey verifies that a user-set key absent from the API response is considered drift. |
| `TestProviderMap_InvalidJSON` | Unit | TestProviderMap_InvalidJSON verifies that providerMap rejects malformed JSON with an error diagnostic. |
| `TestProviderMap_MissingType` | Unit | TestProviderMap_MissingType verifies that providerMap rejects valid JSON that lacks the required "type" key. |
| `TestProviderMap_Valid` | Unit | TestProviderMap_Valid verifies that providerMap parses valid JSON with a type key without error. |
| `TestSchema_IDComputedUseStateForUnknown` | Unit | TestSchema_IDComputedUseStateForUnknown verifies that the "id" attribute is Computed with a UseStateForUnknown plan modifier. |
| `TestSchema_NameRequired` | Unit | TestSchema_NameRequired verifies that "name" is a Required StringAttribute. |
| `TestSchema_ProviderConfigSensitiveRequired` | Unit | TestSchema_ProviderConfigSensitiveRequired verifies that provider_config is Required and Sensitive. |
| `TestUpdatePayload_WireKeyIsProvider` | Unit | TestUpdatePayload_WireKeyIsProvider verifies updatePayload also sends provider_config as the "provider" wire object and includes name. |

## `internal/resources/container`  (34)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccContainer_basic` | Acceptance · conditional skip | TestAccContainer_basic drives the full lifecycle from the design spec: look up a current image version via truenas_container_image, create a container (running=false, autostart=false, an explicit pool — this provider's safety convention for anything container/VM-shaped, see vm's acceptance_test.go), update its description, start it and verify RUNNING, stop it again, import, then destroy + CheckDestroy. |
| `TestContainerDataSourceModel_MatchesSchema` | Unit | TestContainerDataSourceModel_MatchesSchema is the datasource counterpart of TestContainerModel_MatchesSchema. |
| `TestContainerModel_MatchesSchema` | Unit | TestContainerModel_MatchesSchema verifies that every tfsdk tag on ContainerModel has a corresponding attribute in the resource schema, and vice versa. |
| `TestContainerSchema_ComputedOnlyFields` | Unit |  |
| `TestContainerSchema_IDIsComputed` | Unit |  |
| `TestContainerSchema_IdmapIsForceNew` | Unit |  |
| `TestContainerSchema_ImageIsRequiredAndForceNew` | Unit |  |
| `TestContainerSchema_MutableFieldsAreOptionalComputedWithoutForceNew` | Unit |  |
| `TestContainerSchema_NameIsRequiredAndMutable` | Unit |  |
| `TestContainerSchema_NoUnexpectedAttributes` | Unit |  |
| `TestContainerSchema_PoolIsRequiredAndForceNew` | Unit |  |
| `TestCreatePayload_IdmapParsedFromJSON` | Unit |  |
| `TestCreatePayload_InvalidIdmapJSONErrors` | Unit |  |
| `TestCreatePayload_MapsAndScalars` | Unit |  |
| `TestCreatePayload_RequiredFields` | Unit |  |
| `TestCreatePayload_ThreeWayNullableFields` | Unit |  |
| `TestIdmapFromAPI` | Unit | --- idmapFromAPI / idmapResponseValue --------------------------------------- |
| `TestIdmapResponseValue_KnownPlanPreserved` | Unit |  |
| `TestIdmapResponseValue_NullPlanFallsBackToAPI` | Unit |  |
| `TestIdmapResponseValue_UnknownPlanFallsBackToAPI` | Unit |  |
| `TestIsContainerNotStarted` | Unit | --- isContainerAlreadyStopped ---------------------------------------------------- |
| `TestNonNilBoolMap` | Unit |  |
| `TestNonNilStringMap` | Unit | --- nonNilStringMap / nonNilBoolMap ------------------------------------------ |
| `TestPoolFromDataset` | Unit | --- poolFromDataset --------------------------------------------------------- |
| `TestResponseToDataSourceModel` | Unit |  |
| `TestResponseToModel` | Unit |  |
| `TestResponseToModel_DoesNotTouchImageOrIdmap` | Unit |  |
| `TestResponseToModel_NilOptionalPointersBecomeNull` | Unit |  |
| `TestResponseToModel_RunningWhenStateIsRunning` | Unit |  |
| `TestThreeWayString` | Unit | --- threeWayString ------------------------------------------------------ |
| `TestUpdatePayload_NameChangeIncluded` | Unit |  |
| `TestUpdatePayload_NoPoolImageOrIdmap` | Unit |  |
| `TestVersionGateDiagnostics_AtOrAboveFloor` | Unit |  |
| `TestVersionGateDiagnostics_BelowFloor` | Unit | --- versionGateDiagnostics ------------------------------------------------ |

## `internal/resources/container_device`  (23)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccContainerDevice_basic` | Acceptance · conditional skip | TestAccContainerDevice_basic drives the full lifecycle from the design spec: a truenas_container fixture (RandName, running=false, autostart=false, an explicit pool) plus a truenas_dataset fixture (RandName) whose mountpoint becomes a FILESYSTEM device's "source" — container.device.create validates live that "source" must resolve under a pool mount point (probed: omitting it or pointing outside /mnt is rejected with "[EINVAL] attributes.path: The path must reside within a pool mount point"). |
| `TestApiAttributesJSON` | Unit |  |
| `TestAttributesDrifted_APIAddedKey` | Unit |  |
| `TestAttributesDrifted_ChangedValue` | Unit |  |
| `TestAttributesDrifted_Identical` | Unit | --- attributesDrifted ------------------------------------------------------- |
| `TestAttributesDrifted_MissingUserKey` | Unit |  |
| `TestAttributesMap_InvalidJSON` | Unit | --- attributesMap ----------------------------------------------------------- |
| `TestAttributesMap_MissingDtype` | Unit |  |
| `TestAttributesMap_Valid` | Unit |  |
| `TestContainerDeviceDataSourceModel_MatchesSchema` | Unit | TestContainerDeviceDataSourceModel_MatchesSchema is the datasource counterpart of TestContainerDeviceModel_MatchesSchema. |
| `TestContainerDeviceModel_MatchesSchema` | Unit | TestContainerDeviceModel_MatchesSchema verifies that every tfsdk tag on ContainerDeviceModel has a corresponding attribute in the resource schema, and vice versa. |
| `TestCreatePayload_IncludesContainer` | Unit | --- createPayload / updatePayload ------------------------------------------- |
| `TestCreatePayload_InvalidAttributes` | Unit |  |
| `TestResponseToDataSourceModel` | Unit |  |
| `TestResponseToModel` | Unit | --- responseToModel / responseToDataSourceModel / apiAttributesJSON -------- |
| `TestSchema_AttributesRequired` | Unit |  |
| `TestSchema_ContainerRequiresReplace` | Unit |  |
| `TestSchema_IDComputedUseStateForUnknown` | Unit |  |
| `TestSchema_NoUnexpectedAttributes` | Unit |  |
| `TestUpdatePayload_InvalidAttributes` | Unit |  |
| `TestUpdatePayload_OmitsContainer` | Unit |  |
| `TestVersionGateDiagnostics_AtOrAboveFloor` | Unit |  |
| `TestVersionGateDiagnostics_BelowFloor` | Unit | --- versionGateDiagnostics ------------------------------------------------ |

## `internal/resources/container_image`  (9)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccContainerImageDataSource_basic` | Acceptance · conditional skip | TestAccContainerImageDataSource_basic looks up "alpine:3.22:amd64:default" through the truenas_container_image datasource only. |
| `TestAccContainerImageDataSource_notFound` | Acceptance · conditional skip | TestAccContainerImageDataSource_notFound verifies a clean error diagnostic (not a panic/crash) when the requested image name is absent from the registry. |
| `TestContainerImageDataSourceModel_MatchesSchema` | Unit | TestContainerImageDataSourceModel_MatchesSchema verifies that every tfsdk tag on ContainerImageDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestFindRegistryImage` | Unit | --- findRegistryImage -------------------------------------------------------- |
| `TestLatestVersion` | Unit |  |
| `TestVersionGateDiagnostics_AtOrAboveFloor` | Unit |  |
| `TestVersionGateDiagnostics_BelowFloor` | Unit | --- versionGateDiagnostics ------------------------------------------------ |
| `TestVersionStrings_Empty` | Unit |  |
| `TestVersionStrings_PreservesOrder` | Unit | --- versionStrings / latestVersion -------------------------------------------- |

## `internal/resources/cronjob`  (10)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccCronjob_basic` | Acceptance · Tier 1 | TestAccCronjob_basic creates a cron job running /usr/bin/true as root, disabled so nothing is ever actually scheduled to run, checks its attributes, updates its description in place, imports it by numeric id, and verifies destruction via a live cronjob.query. |
| `TestApiPayload_FullySet` | Unit | TestApiPayload_FullySet verifies every optional field is included when known, matching the probed create shape. |
| `TestApiPayload_UnsetOptionalsOmitted` | Unit | TestApiPayload_UnsetOptionalsOmitted verifies that every Optional field is omitted from the payload when null/unknown, leaving only the two Required fields "command" and "user" — so TrueNAS-side defaults take effect. |
| `TestCronjobDataSourceModel_MatchesSchema` | Unit | TestCronjobDataSourceModel_MatchesSchema verifies that every tfsdk tag on CronjobDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestCronjobSchema_IDIsComputed` | Unit | TestCronjobSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestCronjobSchema_OptionalComputedFields` | Unit | TestCronjobSchema_OptionalComputedFields verifies description and the bool flags carrying TrueNAS-side defaults are all Optional+Computed. |
| `TestCronjobSchema_RequiredFields` | Unit | TestCronjobSchema_RequiredFields verifies "command" and "user" are Required, matching cronjob.create's own "required" list. |
| `TestCronjobSchema_ScheduleShape` | Unit | TestCronjobSchema_ScheduleShape verifies the nested "schedule" attribute is Optional+Computed at the top level with Required string sub-fields, matching rsync_task/scrub_task's convention. |
| `TestResponseToDataSourceModel_ProbedShape` | Unit | TestResponseToDataSourceModel_ProbedShape mirrors TestResponseToModel_ProbedShape for the datasource model. |
| `TestResponseToModel_ProbedShape` | Unit | TestResponseToModel_ProbedShape verifies responseToModel against the exact shape observed from a live cronjob.create/query call: no nullable fields, no extra runtime-status fields. |

## `internal/resources/dataset`  (5)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccDataset_basic` | Acceptance · Tier 1 | TestAccDataset_basic creates a ZFS dataset, checks its computed attributes, updates a mutable field (comments), imports it by name, and verifies destruction. |
| `TestDatasetAPIPayloadIncludesVolsizeForVolume` | Unit | TestDatasetAPIPayloadIncludesVolsizeForVolume verifies that a real, non-zero volsize (as used by type=VOLUME datasets) still makes it into the payload - the fix must not suppress legitimate volsize values. |
| `TestDatasetCreateAPIPayloadStillIncludesType` | Unit | TestDatasetCreateAPIPayloadStillIncludesType verifies the create payload (apiPayload) is unaffected by the update-only stripping in updateAPIPayload - type and name must still be present for pool.dataset.create. |
| `TestDatasetUpdateAPIPayloadOmitsCreateOnlyKeys` | Unit | TestDatasetUpdateAPIPayloadOmitsCreateOnlyKeys is a regression test for a live acceptance failure: pool.dataset.update rejected the update payload with "[EINVAL] data.type: Extra inputs are not permitted" because the payload (built from the same apiPayload used for create) still included "type". |
| `TestDatasetUpdateAPIPayloadOmitsVolsizeForFilesystem` | Unit | TestDatasetUpdateAPIPayloadOmitsVolsizeForFilesystem is a regression test for a live acceptance failure: "truenas API error (code 22): 'volsize'". |

## `internal/resources/directoryservices`  (48)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccDirectoryServices_ActiveDirectory` | Acceptance · DS gate | TestAccDirectoryServices_ActiveDirectory drives the singleton truenas_directoryservices resource through a full Active Directory join lifecycle against the disposable TrueNAS 25.10 test VM: join (enable=true, waiting for directoryservices.status to report HEALTHY), update a non-join field (timeout) in place, then destroy — which disables directory services (enable=false) WITHOUT leaving the domain — and verifies the box-side config shows disabled afterward. |
| `TestAccDirectoryServices_IPA` | Acceptance · DS gate | TestAccDirectoryServices_IPA drives the singleton truenas_directoryservices resource through a full FreeIPA join lifecycle against the disposable TrueNAS 25.10 test VM and the disposable tftest-ipa FreeIPA VM (212, realm TFIPA.LAN, domain tfipa.lan, 192.168.1.252): join (enable=true, service_type=IPA, credential_type KERBEROS_USER using the realm's "admin" account, waiting for directoryservices.status to report HEALTHY), update a non-rejoin field (timeout) in place, then destroy — which disables directory services (enable=false) without leaving the realm — and verifies the box-side config shows disabled afterward. |
| `TestAccDirectoryServices_LDAP` | Acceptance · DS gate | TestAccDirectoryServices_LDAP drives the singleton truenas_directoryservices resource through a full plain-LDAP bind lifecycle against the disposable TrueNAS 25.10 test VM and the disposable OpenLDAP VM (VM 211, tftest-ldap): bind (enable=true, service_type=LDAP, credential_type LDAP_PLAIN, waiting for directoryservices.status to report HEALTHY), confirm the seeded LDAP user is visible via user.query, update a non-rejoin field (timeout) in place, then destroy — which disables directory services (enable=false) without unbinding — and verifies the box-side config shows disabled afterward. |
| `TestDirectoryServicesDataSourceModel_MatchesSchema` | Unit | TestDirectoryServicesDataSourceModel_MatchesSchema verifies that every tfsdk tag on DirectoryServicesDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestDirectoryServicesModel_MatchesResourceSchema` | Unit | TestDirectoryServicesModel_MatchesResourceSchema is the resource-side counterpart: every tfsdk tag on DirectoryServicesModel must have a corresponding attribute in the resource schema, and vice versa. |
| `TestNeedsServiceTypeReset` | Unit | TestNeedsServiceTypeReset covers the predicate that gates DirectoryServicesResource.resetStaleServiceType (resource.go) — see that method's doc comment for the confirmed-live middleware defect this exists to work around (switching service_type away from a previous ACTIVEDIRECTORY/IPA join leaves a stale kerberos_realm/credential in place unless this singleton is fully cleared first). |
| `TestResponseToDataSourceModel_StatusFields` | Unit | TestResponseToDataSourceModel_StatusFields verifies that status/status_msg are mapped from the separate directoryservices.status response. |
| `TestResponseToModel_ADIdmapReadBack` | Unit | TestResponseToModel_ADIdmapReadBack verifies the mapper decodes configuration.idmap into the new explicit ActiveDirectoryConfigModel.Idmap object, replacing the old verbatim-JSON round-trip. |
| `TestResponseToModel_IPAConfig` | Unit | TestResponseToModel_IPAConfig verifies the mapper fills ConfigurationIPA (and leaves AD/LDAP null) when service_type is IPA. |
| `TestResponseToModel_LDAPConfig` | Unit | TestResponseToModel_LDAPConfig verifies the mapper fills ConfigurationLDAP (and leaves AD/IPA null) when service_type is LDAP, including search_bases/attribute_maps sub-objects. |
| `TestResponseToModel_MapsPlainFields` | Unit | TestResponseToModel_MapsPlainFields verifies responseToModel against the shape probed from a live directoryservices.config call while joined to Active Directory. |
| `TestResponseToModel_NeverFillsPassword` | Unit | TestResponseToModel_NeverFillsPassword is the "mapper never fills password" requirement from the task-8 brief: even starting from a model whose credential already carries a password, and mapping in an API response, responseToModel must leave Credential completely untouched (the API response is never decoded into anything password-shaped in the first place — see directoryServicesAPI's doc comment). |
| `TestResponseToModel_NullServiceTypeAndConfiguration` | Unit | TestResponseToModel_NullServiceTypeAndConfiguration verifies the never-joined shape confirmed live on a disposable box: {"configuration":null,"credential":null,"enable":false, "enable_account_cache":true,"enable_dns_updates":true,"id":1, "kerberos_realm":null,"service_type":null,"timeout":10}. |
| `TestSchema_ConfigurationActiveDirectoryOptionalComputed` | Unit | TestSchema_ConfigurationActiveDirectoryOptionalComputed verifies the nested AD config block is Optional+Computed (nullable on the wire, and safe to be Computed since it has no WriteOnly children), and that its required leaf fields (hostname, domain) are Required. |
| `TestSchema_ConfigurationLDAPAndIPAOptionalComputed` | Unit | TestSchema_ConfigurationLDAPAndIPAOptionalComputed verifies the two new discriminated-union blocks exist as Optional+Computed SingleNestedAttribute with their Required leaf fields intact. |
| `TestSchema_CredentialLDAPVariantFields` | Unit | TestSchema_CredentialLDAPVariantFields verifies the credential block's new LDAP/Kerberos-principal fields: bindpw is Sensitive+WriteOnly (like password); binddn/client_certificate/principal are plain Optional (persisted normally, not sensitive). |
| `TestSchema_CredentialPasswordIsWriteOnly` | Unit | TestSchema_CredentialPasswordIsWriteOnly verifies the credential nested block's password attribute is Sensitive+WriteOnly (never persisted in state), and that the parent "credential" attribute itself is NOT Computed — the terraform-plugin-framework forbids a Computed nested attribute from containing a WriteOnly child. |
| `TestSchema_EnableIsRequired` | Unit | TestSchema_EnableIsRequired verifies "enable" is a plain Required bool: the resource's core purpose is to toggle directory-service enablement, so it should never be silently defaulted. |
| `TestSchema_IDIsComputed` | Unit | TestSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestSchema_IdmapBlock` | Unit | TestSchema_IdmapBlock verifies configuration_activedirectory.idmap is Optional+Computed (back-compat: unset omits it from the payload, and a read-back populates it — see updatePayload's doc comment), and that idmap_domain's idmap_backend is restricted to the two backends this provider version models (AD, RID). |
| `TestSchema_ServiceTypeAllowsThreeTypes` | Unit | TestSchema_ServiceTypeAllowsThreeTypes verifies that "service_type" carries a validator restricting it to the three service types the middleware itself supports: ACTIVEDIRECTORY, LDAP, IPA. |
| `TestSchema_TimeoutBetween5And60` | Unit | TestSchema_TimeoutBetween5And60 verifies "timeout" carries a range validator matching the probed minimum/maximum (5-60 seconds). |
| `TestUpdatePayload_AD_IdmapDomainADOmitsSSSDCompat` | Unit | TestUpdatePayload_AD_IdmapDomainADOmitsSSSDCompat verifies the converse of TestUpdatePayload_AD_IdmapDomainRIDOmitsADOnlyFields: an AD idmap_domain payload includes schema_mode/unix_primary_group/unix_nss_info but never sssd_compat (RID-only). |
| `TestUpdatePayload_AD_IdmapDomainADRejectsSSSDCompat` | Unit | TestUpdatePayload_AD_IdmapDomainADRejectsSSSDCompat verifies the converse preflight added for finding #1: a user-set sssd_compat under idmap_backend="AD" is rejected before any API call (sssd_compat is RID-only). |
| `TestUpdatePayload_AD_IdmapDomainADRequiresSchemaMode` | Unit | TestUpdatePayload_AD_IdmapDomainADRequiresSchemaMode verifies the idmap_domain preflight: idmap_backend "AD" requires schema_mode. |
| `TestUpdatePayload_AD_IdmapDomainRIDOmitsADOnlyFields` | Unit | TestUpdatePayload_AD_IdmapDomainRIDOmitsADOnlyFields verifies that a RID idmap_domain payload never includes unix_primary_group/unix_nss_info (AD-only fields), even when the model holds known (non-null) Bool values for them — the state their read-back path always populates (see adIdmapToModel: it decodes them unconditionally via types.BoolValue, never types.BoolNull, regardless of the actual backend). |
| `TestUpdatePayload_AD_IdmapDomainRIDRejectsADOnlyFields` | Unit | TestUpdatePayload_AD_IdmapDomainRIDRejectsADOnlyFields verifies the preflight added for finding #1: a user-set AD-only idmap_domain field (schema_mode/unix_primary_group/unix_nss_info) under idmap_backend="RID" is rejected with a diagnostic before any API call, rather than being silently dropped by buildADConfigPayload's backend gating (which would otherwise let the join succeed and only surface as a "Provider produced inconsistent result after apply" error once read-back decodes the dropped field back to false). |
| `TestUpdatePayload_AD_IdmapDomainUnknownBackendOmitsBackendSpecificFields` | Unit | TestUpdatePayload_AD_IdmapDomainUnknownBackendOmitsBackendSpecificFields covers finding #2: an idmap_domain read back from an out-of-band LDAP/RFC2307 config (idmap_backend values this provider doesn't model as a full variant — see idmapDomainAttrTypes's doc comment) carries forward into the plan via UseStateForUnknown. |
| `TestUpdatePayload_AD_IdmapFromModel` | Unit | TestUpdatePayload_AD_IdmapFromModel verifies that an explicitly-set idmap block (builtin + idmap_domain RID) is reconstructed byte-for-byte into the outgoing "configuration.idmap" payload. |
| `TestUpdatePayload_AD_IdmapOmittedWhenUnset` | Unit | TestUpdatePayload_AD_IdmapOmittedWhenUnset verifies that a null/unset idmap block is omitted entirely from the outgoing configuration payload (back-compat: the existing AD acceptance test config, which never sets idmap, must remain valid — see enabledModel/validADConfig, which leave Idmap null). |
| `TestUpdatePayload_ConfigurationSiteAndOUThreeWay` | Unit | TestUpdatePayload_ConfigurationSiteAndOUThreeWay verifies the configuration_activedirectory nested block's site/computer_account_ou follow the same three-way clear convention as kerberos_realm. |
| `TestUpdatePayload_CredentialVariants` | Unit | TestUpdatePayload_CredentialVariants table-tests credentialPayload for all five probed credential_type values. |
| `TestUpdatePayload_DisableIgnoresJoinFields` | Unit | TestUpdatePayload_DisableIgnoresJoinFields is a regression test for a bug found live against a TrueNAS 25.10 box: TrueNAS rejects directoryservices.update with "[EINVAL] directoryservices.update.configuration: Permitted changes while directory services are enabled are limited to account caching, DNS updates, and timeouts" whenever "configuration" differs at all from what is currently persisted while directory services are (or would stay) enabled. |
| `TestUpdatePayload_DisableShape_MinimalWhenAllScalarsUnset` | Unit | TestUpdatePayload_DisableShape_MinimalWhenAllScalarsUnset verifies the absolute minimum disable payload: just {"enable": false}. |
| `TestUpdatePayload_DisableShape_OnlyScalars` | Unit | TestUpdatePayload_DisableShape_OnlyScalars verifies that whenever enable=false, the payload is exactly {enable, enable_account_cache, enable_dns_updates, timeout} (each still individually guarded by null/unknown) — confirmed live: TrueNAS accepts a bare {"enable": false} to disable an active join. |
| `TestUpdatePayload_EnableRequiresCredentialAndConfiguration` | Unit | TestUpdatePayload_EnableRequiresCredentialAndConfiguration verifies that enable=true without a valid credential and/or configuration_activedirectory fails fast with a diagnostic, rather than sending an incomplete payload TrueNAS would reject anyway with a much less clear EINVAL. |
| `TestUpdatePayload_EnableShape_FullPayload` | Unit | TestUpdatePayload_EnableShape_FullPayload verifies that enable=true with a valid credential and configuration produces the full join-shape payload confirmed live: service_type, credential, and configuration are all present alongside the scalar fields. |
| `TestUpdatePayload_ExactlyOneConfigBlock_NoneSet` | Unit | TestUpdatePayload_ExactlyOneConfigBlock_NoneSet verifies the preflight error when enable=true and service_type is set, but no configuration_* block is set at all. |
| `TestUpdatePayload_ExactlyOneConfigBlock_ServiceTypeMismatch` | Unit | TestUpdatePayload_ExactlyOneConfigBlock_ServiceTypeMismatch verifies the preflight error when exactly one configuration_* block is set, but it doesn't match service_type. |
| `TestUpdatePayload_ExactlyOneConfigBlock_TwoSet` | Unit | TestUpdatePayload_ExactlyOneConfigBlock_TwoSet verifies the preflight error when more than one configuration_* block is set simultaneously. |
| `TestUpdatePayload_IPA_Payload` | Unit | TestUpdatePayload_IPA_Payload verifies the IPA configuration payload shape: target_server/hostname/domain/basedn always present, smb_domain omitted when unset (the normal case — it's server-detected during join). |
| `TestUpdatePayload_KerberosRealmThreeWay` | Unit | TestUpdatePayload_KerberosRealmThreeWay verifies the null/omitted, unknown/omitted, explicit-empty/nil, and value/value cases for kerberos_realm (only sent in the enable=true payload shape), mirroring network_config's nameserver1-3 three-way guard. |
| `TestUpdatePayload_LDAP_FullPayload` | Unit | TestUpdatePayload_LDAP_FullPayload verifies the LDAP configuration payload shape when search_bases and attribute_maps ARE set: only the leaf fields the caller actually set are included, following the same three-way clearable-string convention as the rest of the package. |
| `TestUpdatePayload_LDAP_MinimalPayload` | Unit | TestUpdatePayload_LDAP_MinimalPayload verifies the LDAP configuration payload shape when search_bases/attribute_maps are left unset: they must be omitted entirely (server defaults apply), and credential_type LDAP_ANONYMOUS produces a bare {"credential_type":"LDAP_ANONYMOUS"}. |
| `TestUpdatePayload_ReusesExistingKerberosPrincipal` | Unit | TestUpdatePayload_ReusesExistingKerberosPrincipal is a regression test for a bug found live against a TrueNAS 25.10 box: TrueNAS swaps a raw KERBEROS_USER admin credential used for the initial join into a KERBEROS_PRINCIPAL backed by the new machine account's own keytab, and then REJECTS resending the raw KERBEROS_USER credential on a later call (the disabling call specifically failed with "[EINVAL] directoryservices.update.credential.credential_type: Kerberos user credentials may not be stored for disabled directory services...", because the immediately preceding update had re-stored a raw KERBEROS_USER). |
| `TestUpdatePayload_TrustedDomainsIgnoredForOtherServiceType` | Unit | TestUpdatePayload_TrustedDomainsIgnoredForOtherServiceType verifies that existing.Configuration.TrustedDomains is only round-tripped when the existing configuration's own top-level ServiceType is ACTIVEDIRECTORY (defensive: trusted_domains doesn't exist on the IPA/LDAP variants). |
| `TestUpdatePayload_TrustedDomainsRoundTrip` | Unit | TestUpdatePayload_TrustedDomainsRoundTrip verifies that trusted_domains from a previously-fetched directoryServicesAPI is copied verbatim into the outgoing "configuration" map — required so an in-place update (e.g. |
| `TestValidateCredential_MissingFields` | Unit | TestValidateCredential_MissingFields table-tests the per-credential-type required-field diagnostics. |

## `internal/resources/docker_config`  (18)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccDockerConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccDockerConfigDataSource_basic reads the current TrueNAS Docker configuration through the truenas_docker_config datasource only. |
| `TestAccDockerConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccDockerConfig_setAndRestore drives the singleton truenas_docker_config resource's "enable_image_updates" field through its opposite value and back to the value read from the box before the test ran, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies Delete's diagnostic builder returns exactly one warning and never touches a client. |
| `TestDockerConfigDataSourceModel_MatchesSchema` | Unit | TestDockerConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on DockerConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestDockerConfigSchema_AddressPoolsNestedFieldsRequired` | Unit | TestDockerConfigSchema_AddressPoolsNestedFieldsRequired verifies the nested address_pools entry has base/size both Required, matching docker.update's accepts schema (both required within each entry). |
| `TestDockerConfigSchema_DatasetIsComputedOnly` | Unit | TestDockerConfigSchema_DatasetIsComputedOnly verifies "dataset" is read-only: docker.update does not accept it. |
| `TestDockerConfigSchema_IDIsComputed` | Unit | TestDockerConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestDockerConfigSchema_MigrateApplicationsExcluded` | Unit | TestDockerConfigSchema_MigrateApplicationsExcluded verifies the schema never exposes "migrate_applications": it's an apply-time action flag, not persisted config (see resourceSchema's Description). |
| `TestDockerConfigSchema_SettableFieldsAreOptionalComputed` | Unit | TestDockerConfigSchema_SettableFieldsAreOptionalComputed verifies the fields docker.update actually accepts (per the probe) are all Optional+Computed. |
| `TestHasUnifiedRegistryMirrors_KeyPresenceDetection` | Unit | TestHasUnifiedRegistryMirrors_KeyPresenceDetection verifies shape detection: an empty-but-present "registry_mirrors" JSON array (a non-nil, zero-length Go slice) is still detected as the unified shape, distinct from the key being entirely absent (nil slice). |
| `TestResponseToDataSourceModel` | Unit | TestResponseToDataSourceModel mirrors TestResponseToModel_AddressPoolsNesting for the datasource model. |
| `TestResponseToModel_AddressPoolsNesting` | Unit | TestResponseToModel_AddressPoolsNesting verifies address_pools decodes into the nested {base,size} model shape. |
| `TestResponseToModel_NullablePoolAndDataset` | Unit | TestResponseToModel_NullablePoolAndDataset verifies pool/dataset decode to null when the API returns null (docker unconfigured), matching the mail/ups precedent for detecting "unconfigured" in acceptance PreCheck. |
| `TestResponseToModel_RegistryMirrors_SplitShape` | Unit | TestResponseToModel_RegistryMirrors_SplitShape verifies the TrueNAS 25.10 split secure/insecure string arrays are translated into the unified {url,insecure} shape, secure entries first. |
| `TestResponseToModel_RegistryMirrors_UnifiedShape` | Unit | TestResponseToModel_RegistryMirrors_UnifiedShape verifies the TrueNAS 26.0+ unified registry_mirrors array decodes directly. |
| `TestUpdatePayload_AllFieldsSet_Split` | Unit | TestUpdatePayload_AllFieldsSet_Split verifies registry_mirrors is split into secure_registry_mirrors/insecure_registry_mirrors when registryMirrorsUnified=false (TrueNAS 25.10 wire shape). |
| `TestUpdatePayload_AllFieldsSet_Unified` | Unit | TestUpdatePayload_AllFieldsSet_Unified25 verifies every non-nvidia field is included with the unified (TrueNAS 26.0+) registry_mirrors shape when registryMirrorsUnified=true. |
| `TestUpdatePayload_UnsetOptionalsOmitted` | Unit | TestUpdatePayload_UnsetOptionalsOmitted verifies every field is omitted when null/unknown, so the current TrueNAS-side value is left unchanged. |

## `internal/resources/docker_network`  (6)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccDockerNetworkDataSource_basic` | Acceptance · conditional skip | TestAccDockerNetworkDataSource_basic looks up Docker's built-in "bridge" network by name through the truenas_docker_network datasource. |
| `TestDockerNetworkDataSourceModel_MatchesSchema` | Unit | TestDockerNetworkDataSourceModel_MatchesSchema verifies that every tfsdk tag on DockerNetworkDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestResponseToDataSourceModel_AllFieldsNull` | Unit | TestResponseToDataSourceModel_AllFieldsNull verifies every nullable top-level field (per the probed docker.network.query schema, where id/ name/driver/scope/short_id/created are each anyOf string-or-null) decodes cleanly to a null string rather than panicking or producing "". |
| `TestResponseToDataSourceModel_FullNetwork` | Unit | TestResponseToDataSourceModel_FullNetwork verifies decoding a real bridge network with ipam.config populated and labels set (probed live shape, "ix-plex_default" on TrueNAS 26.0 — see task-1-report.md). |
| `TestResponseToDataSourceModel_NilIPAM` | Unit | TestResponseToDataSourceModel_NilIPAM verifies a wholly-null "ipam" field (per the probed nullable schema) decodes to a null ipam object. |
| `TestResponseToDataSourceModel_NullIPAMConfig` | Unit | TestResponseToDataSourceModel_NullIPAMConfig verifies the "none" network shape: ipam is present (driver set) but config is null (probed live) — this must decode to a non-null ipam object containing an empty config list, not a null ipam object. |

## `internal/resources/enclosure`  (7)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccEnclosureDataSource_basic` | Acceptance · HA gate | TestAccEnclosureDataSource_basic reads a real enclosure's descriptor through the truenas_enclosure datasource only. |
| `TestAccEnclosureDataSource_notFound` | Acceptance · HA gate | TestAccEnclosureDataSource_notFound verifies the id-filtered enclosure2.query call is surfaced as a clean, informative "not found" error rather than a crash or a confusing empty-object read — the same outcome probed live on the cross-release 26.0 box (no enclosure hardware/ license there: enclosure2.query returns an empty array for every id). |
| `TestEnclosureDataSourceModel_MatchesSchema` | Unit | TestEnclosureDataSourceModel_MatchesSchema verifies that every tfsdk tag on EnclosureDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestEnclosureQueryArgs` | Unit | --- enclosureQueryArgs -------------------------------------------------- |
| `TestResponseToDataSourceModel` | Unit |  |
| `TestResponseToDataSourceModel_LabelIndependentOfName` | Unit | TestResponseToDataSourceModel_LabelIndependentOfName pins the decisive live-probed evidence that "label" and "name" are independent fields (see enclosureAPI's doc comment): a customized label must decode distinctly from an unrelated name. |
| `TestResponseToDataSourceModel_NilStatus` | Unit | TestResponseToDataSourceModel_NilStatus verifies the nil-guard: a nil api.Status must decode to a non-null, empty types.List, not a null one (matching the house convention for API-returned lists, e.g. |

## `internal/resources/enclosure_label`  (11)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccEnclosureLabel_setAndRestore` | Acceptance · HA gate | TestAccEnclosureLabel_setAndRestore drives the truenas_enclosure_label resource's "label" field through a fresh RandName value, verifies it round-trips via Terraform (apply + import), then lets Terraform's own implicit end-of-test Destroy run — which, per this resource's own restore-on-destroy contract (see schema.go's resourceSchema Description), must independently restore the enclosure's ORIGINAL label. |
| `TestCaptureOriginalLabel_HandlesSpecialCharacters` | Unit | TestCaptureOriginalLabel_HandlesSpecialCharacters verifies a label containing characters that are meaningful in raw JSON (quotes, backslash, unicode) round-trips correctly through json.Marshal/Unmarshal — private state values must be valid JSON+UTF-8 (framework requirement), so this pins that captureOriginalLabel never hand-builds the JSON string itself. |
| `TestCaptureOriginalLabel_PropagatesSetKeyError` | Unit |  |
| `TestCaptureOriginalLabel_StoresJSONEncodedLabel` | Unit |  |
| `TestEnclosureLabelSchema_IDRequiredForceNew` | Unit | TestEnclosureLabelSchema_IDRequiredForceNew pins the identity contract: "id" identifies a pre-existing enclosure (enclosure.label.set takes it as a positional argument), so it must be Required and force replacement rather than being renamed in place. |
| `TestEnclosureLabelSchema_LabelRequiredNotComputed` | Unit |  |
| `TestEnclosureLabelSchema_NameComputedOnly` | Unit |  |
| `TestEnclosureLabelSchema_NoUnexpectedAttributes` | Unit |  |
| `TestEnclosureQueryArgs` | Unit | --- enclosureQueryArgs -------------------------------------------------- |
| `TestResponseToModel` | Unit |  |
| `TestResponseToModel_LabelIndependentOfName` | Unit |  |

## `internal/resources/failover_config`  (17)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccFailoverConfigDataSource_basic` | Acceptance · HA gate | TestAccFailoverConfigDataSource_basic reads the current failover configuration and live HA status through the truenas_failover_config datasource only. |
| `TestAccFailoverConfig_setAndRestore` | Acceptance · HA gate | TestAccFailoverConfig_setAndRestore drives the singleton truenas_failover_config resource's "timeout" field (the ONLY field this provider's own tests ever touch — see model.go's updatePayload doc comment for why "disabled"/"master" are never exercised by a committed test) through a different value, then back to the value read from the box before the test ran, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | --- deleteWarningDiagnostics -------------------------------------------- |
| `TestFailoverConfigDataSourceModel_MatchesSchema` | Unit | TestFailoverConfigDataSourceModel_MatchesSchema is the datasource counterpart of TestFailoverConfigModel_MatchesSchema. |
| `TestFailoverConfigModel_MatchesSchema` | Unit | TestFailoverConfigModel_MatchesSchema verifies that every tfsdk tag on FailoverConfigModel has a corresponding attribute in the resource schema, and vice versa. |
| `TestFailoverConfigSchema_IDIsComputed` | Unit |  |
| `TestFailoverConfigSchema_NoUnexpectedAttributes` | Unit |  |
| `TestFailoverConfigSchema_WritableFields` | Unit | TestFailoverConfigSchema_WritableFields pins the schema-level half of this resource's safety contract: "disabled", "master", and "timeout" must be Optional+Computed (user-writable), matching failover.update's own accepts schema (probed live: all three are optional on the "data" payload, on both TrueNAS 25.10.4 HA and 26.0). |
| `TestNonNilStrings` | Unit | --- nonNilStrings ----------------------------------------------------------- |
| `TestResponseToDataSourceModel` | Unit | --- responseToDataSourceModel --------------------------------------------- |
| `TestResponseToDataSourceModel_NilReasonsIsKnownEmptyList` | Unit |  |
| `TestResponseToDataSourceModel_WithReasons` | Unit |  |
| `TestResponseToModel` | Unit |  |
| `TestResponseToModel_NonDefaultValues` | Unit |  |
| `TestUpdatePayload_AllFieldsSet` | Unit |  |
| `TestUpdatePayload_AllUnsetOmitsEverything` | Unit | --- updatePayload ---------------------------------------------------------  This is the load-bearing safety-critical test group: no committed code path exercised by any test in this repository may ever cause failover.update to be called with a "disabled" or "master" value that differs from the box's live state. |
| `TestUpdatePayload_OnlyTimeoutSet` | Unit |  |

## `internal/resources/filesystem_acl`  (20)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccFilesystemAcl_basic` | Acceptance · Tier 1 | TestAccFilesystemAcl_basic pre-creates an NFS4-acltype dataset fixture directly via the raw API (in PreCheck, cleaned up via t.Cleanup) rather than through truenas_dataset: this provider's probe pool has "tank"'s root acltype=POSIX AND aclmode=DISCARD set LOCAL (probed live), and TrueNAS rejects `pool.dataset.create({"acltype":"NFSV4"})` with EINVAL ("aclmode may not be set for NFSv4 acl type") when the inherited aclmode is DISCARD - truenas_dataset does not expose "aclmode" to override this, so an explicit aclmode="PASSTHROUGH" (verified live) must be sent alongside acltype, which only the raw pool.dataset.create call below can do. |
| `TestCanonicalEntriesJSON_InvalidJSON` | Unit |  |
| `TestDeleteACLPayload` | Unit | TestDeleteACLPayload verifies the exact stripacl=true payload shape (probed live, confirmed a clean strip on both NFS4 and POSIX1E). |
| `TestDeleteWarningDiagnostics` | Unit |  |
| `TestEntriesDrifted_NullVsMinusOneNotDrift` | Unit | TestEntriesDrifted_NullVsMinusOneNotDrift verifies the two id sentinels don't trigger drift against each other, matching the exact quirk probed live on filesystem.getacl. |
| `TestEntriesDrifted_RealChangeIsDrift` | Unit | TestEntriesDrifted_RealChangeIsDrift verifies an actual permission change is still detected as drift. |
| `TestEntriesNormalized_PosixMaskRequired` | Unit | TestEntriesNormalized_PosixMaskRequired is a documentation-level unit test (POSIX1E is not live-testable on this provider's probe tank/NFS4 fixture path per the task brief - see schema.go) covering the wire quirk discovered live: a POSIX1E ACL with a named USER/GROUP entry requires a MASK entry. |
| `TestEntriesNormalized_StripsIDSentinels` | Unit | TestEntriesNormalized_StripsIDSentinels verifies both observed "no id" sentinels (null and -1, probed live via filesystem.getacl) are dropped, and a real USER/GROUP id (e.g. |
| `TestFilesystemAclDataSourceModel_MatchesSchema` | Unit | TestFilesystemAclDataSourceModel_MatchesSchema verifies that every tfsdk tag on FilesystemAclDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestFilesystemAclModel_MatchesSchema` | Unit | TestFilesystemAclModel_MatchesSchema is the same check for the resource model/schema pair. |
| `TestFilesystemAclSchema_ACLTypeIsComputed` | Unit | TestFilesystemAclSchema_ACLTypeIsComputed verifies "acltype" is Computed-only: this resource never sets it, only exposes what filesystem.getacl reports. |
| `TestFilesystemAclSchema_EntriesRequired` | Unit | TestFilesystemAclSchema_EntriesRequired verifies "entries" is Required (matching truenas_acl_template's "acl" field precedent). |
| `TestFilesystemAclSchema_IDIsComputed` | Unit | TestFilesystemAclSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestFilesystemAclSchema_PathRequiredForceNew` | Unit | TestFilesystemAclSchema_PathRequiredForceNew verifies "path" is Required and forces resource replacement on change, matching truenas_filesystem_ permissions' own path attribute. |
| `TestFilesystemAclSchema_RecursiveTraverseOptionalOnly` | Unit | TestFilesystemAclSchema_RecursiveTraverseOptionalOnly verifies "recursive"/"traverse" are apply-time-only (Optional, never Computed), matching truenas_filesystem_permissions' own options. |
| `TestFilesystemAclSchema_UIDGIDOptionalComputed` | Unit | TestFilesystemAclSchema_UIDGIDOptionalComputed verifies "uid"/"gid" carry the setperm-style "leave unchanged when omitted, always read back" convention (Optional+Computed), matching truenas_filesystem_permissions. |
| `TestResponseToDataSourceModel` | Unit |  |
| `TestResponseToModel` | Unit |  |
| `TestSetaclPayload_FullySet` | Unit |  |
| `TestSetaclPayload_UIDGIDOmittedWhenUnset` | Unit |  |

## `internal/resources/filesystem_permissions`  (14)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccFilesystemPermissions_basic` | Acceptance · Tier 1 | TestAccFilesystemPermissions_basic creates a dataset fixture plus a fixture user and group (for uid/gid), sets mode 0750 + those uid/gid via truenas_filesystem_permissions, stat-verifies the result server-side, updates the mode to 0770 in place, imports by path (ignoring the apply-time-only recursive/traverse options), then removes ONLY the filesystem_permissions resource from the config (destroying it while leaving the dataset/user/group fixtures standing) and asserts — this is the documented Delete semantic — that the dataset still exists AND its mode is STILL 0770: destroy does not revert anything, it only forgets Terraform state. |
| `TestDeleteWarningDiagnostics` | Unit |  |
| `TestFilesystemPermissionsDataSourceModel_MatchesSchema` | Unit | TestFilesystemPermissionsDataSourceModel_MatchesSchema verifies that every tfsdk tag on FilesystemPermissionsDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestHasWrite` | Unit |  |
| `TestModeRegexp` | Unit | TestModeRegexp verifies the validator regexp accepts 3-4 octal digits and rejects everything else. |
| `TestPermModeString` | Unit | TestPermModeString verifies the full st_mode -> permission-bits-only octal string conversion, using the exact value observed probing filesystem.stat live after filesystem.setperm mode="0750" on a directory (0o40750 = decimal 16872). |
| `TestResponseToDataSourceModel` | Unit |  |
| `TestResponseToModel` | Unit |  |
| `TestSchema_IDIsComputed` | Unit | TestSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestSchema_ModeUidGidOptionalComputed` | Unit | TestSchema_ModeUidGidOptionalComputed verifies mode/uid/gid are Optional+Computed: settable, but always read back from filesystem.stat (drift-checked on every refresh). |
| `TestSchema_PathIsRequiredForceNew` | Unit | TestSchema_PathIsRequiredForceNew verifies "path" is Required and forces replacement on change (this resource is keyed by path). |
| `TestSchema_RecursiveTraverseAreNotComputed` | Unit | TestSchema_RecursiveTraverseAreNotComputed verifies "recursive" and "traverse" are plain Optional (never echoed back into state, per the brief and the resource's Read implementation). |
| `TestSetpermPayload_FullySet` | Unit |  |
| `TestSetpermPayload_OmitsUnsetModeUidGid` | Unit | TestSetpermPayload_OmitsUnsetModeUidGid verifies mode/uid/gid are omitted entirely (not sent as explicit null) when unset, so filesystem.setperm's "leave unchanged" default applies rather than the provider clobbering an out-of-band value. |

## `internal/resources/ftp_config`  (13)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccFTPConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccFTPConfigDataSource_basic reads the current TrueNAS FTP configuration through the truenas_ftp_config datasource only. |
| `TestAccFTPConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccFTPConfig_setAndRestore drives the singleton truenas_ftp_config resource's "banner" field (a cosmetic, low-risk login banner string) through a test value and back to the value read from the box before the test ran, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: FTPConfigResource.Delete calls only this pure function. |
| `TestFTPConfigDataSourceModel_MatchesSchema` | Unit | TestFTPConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on FTPConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestFTPConfigSchema_AllFieldsOptionalComputed` | Unit | TestFTPConfigSchema_AllFieldsOptionalComputed verifies that every non-id field is Optional+Computed with a plan modifier, matching the "all fields Optional+Computed + UseStateForUnknown" contract in the task brief. |
| `TestFTPConfigSchema_IDIsComputed` | Unit | TestFTPConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestResponseToDataSourceModel_NullableFieldsNilBecomeZero` | Unit | TestResponseToDataSourceModel_NullableFieldsNilBecomeZero mirrors the same nil-to-zero handling for the datasource model. |
| `TestResponseToModel_NullableFieldsNilBecomeZero` | Unit | TestResponseToModel_NullableFieldsNilBecomeZero verifies that nil ssltls_certificate/anonpath from the API map to the zero value (0 / "") in the model, matching the live config sanity sample where both are null. |
| `TestResponseToModel_NullableFieldsSet` | Unit | TestResponseToModel_NullableFieldsSet verifies that non-nil ssltls_certificate/anonpath from the API are carried through as-is. |
| `TestUpdatePayload_AllKnownFieldsSent` | Unit | TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes every guarded plain field when its model value is known. |
| `TestUpdatePayload_AnonPathThreeWay` | Unit | TestUpdatePayload_AnonPathThreeWay verifies the three-way handling for the nullable anonpath field: null/unknown is omitted, an explicit "" is sent as nil (clearing the path), and any other value is sent as-is. |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits every field whose model value is null or unknown, across all field types including the two nullable fields. |
| `TestUpdatePayload_SSLTLSCertificateThreeWay` | Unit | TestUpdatePayload_SSLTLSCertificateThreeWay verifies the three-way handling for the nullable ssltls_certificate field: null/unknown is omitted, an explicit 0 is sent as nil (clearing the certificate), and any other value is sent as-is. |

## `internal/resources/group`  (7)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccGroup_basic` | Acceptance · Tier 1 | TestAccGroup_basic creates a local group, checks its attributes, flips the smb flag and updates sudo_commands in place, imports it by its numeric id, and verifies destruction. |
| `TestGroupCreatePayload` | Unit | TestGroupCreatePayload verifies that createPayload includes gid and name, and uses the "name" key (not "group"). |
| `TestGroupCreatePayload_NullGID` | Unit | TestGroupCreatePayload_NullGID verifies that createPayload does NOT include gid when GID is null, avoiding the root group conflict (gid: 0). |
| `TestGroupResponseToModel` | Unit | TestGroupResponseToModel verifies responseToModel populates all fields correctly. |
| `TestGroupResponseToModel_NilSlices` | Unit | TestGroupResponseToModel_NilSlices verifies nil slices map to empty lists. |
| `TestGroupSchema` | Unit | TestGroupSchema verifies key schema attributes. |
| `TestGroupUpdatePayload` | Unit | TestGroupUpdatePayload verifies that updatePayload omits gid and name. |

## `internal/resources/init_shutdown_script`  (16)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccInitShutdownScript_basic` | Acceptance · Tier 1 | TestAccInitShutdownScript_basic creates a COMMAND-type init/shutdown script running /usr/bin/true at POSTINIT, disabled so nothing is ever actually scheduled to run, checks its attributes, updates its comment in place, imports it by numeric id, and verifies destruction via a live initshutdownscript.query. |
| `TestApiPayload_COMMANDEmptyCommand` | Unit | TestApiPayload_COMMANDEmptyCommand verifies an explicitly empty-string command is treated the same as unset (rejected), since an empty command is indistinguishable from "not configured" and the server itself would reject it. |
| `TestApiPayload_COMMANDMissingCommand` | Unit | TestApiPayload_COMMANDMissingCommand verifies the conditional-required preflight rejects type=COMMAND without a non-empty "command", matching the server-side EINVAL probed live ("init_shutdown_script_create.command: This field is required"). |
| `TestApiPayload_COMMANDType` | Unit | TestApiPayload_COMMANDType verifies the payload built for a COMMAND-type task: "command" is included, matching the probed create shape. |
| `TestApiPayload_SCRIPTMissingScript` | Unit | TestApiPayload_SCRIPTMissingScript verifies the symmetric conditional-required preflight for type=SCRIPT without a non-empty "script". |
| `TestApiPayload_SCRIPTType` | Unit | TestApiPayload_SCRIPTType verifies the payload built for a SCRIPT-type task: "script" is included, "command" omitted. |
| `TestApiPayload_UnsetOptionalsOmitted` | Unit | TestApiPayload_UnsetOptionalsOmitted verifies that every Optional field is omitted from the payload when null/unknown, leaving only "type" and "when" — so TrueNAS-side defaults take effect. |
| `TestInitShutdownScriptDataSourceModel_MatchesSchema` | Unit | TestInitShutdownScriptDataSourceModel_MatchesSchema verifies that every tfsdk tag on InitShutdownScriptDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestInitShutdownScriptSchema_CommandScriptAreOptionalComputed` | Unit | TestInitShutdownScriptSchema_CommandScriptAreOptionalComputed verifies "command" and "script" are Optional+Computed, not Required — the conditional requiredness is enforced in apiPayload, not the schema, since which one is required depends on "type". |
| `TestInitShutdownScriptSchema_IDIsComputed` | Unit | TestInitShutdownScriptSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestInitShutdownScriptSchema_OptionalComputedFields` | Unit | TestInitShutdownScriptSchema_OptionalComputedFields verifies enabled, timeout, and comment are all Optional+Computed. |
| `TestInitShutdownScriptSchema_RequiredFields` | Unit | TestInitShutdownScriptSchema_RequiredFields verifies "type" and "when" are Required, matching initshutdownscript.create's own "required" list. |
| `TestInitShutdownScriptSchema_TypeWhenHaveEnumValidators` | Unit | TestInitShutdownScriptSchema_TypeWhenHaveEnumValidators verifies "type" and "when" are constrained to the enum values probed live. |
| `TestResponseToDataSourceModel_ProbedShape` | Unit | TestResponseToDataSourceModel_ProbedShape mirrors TestResponseToModel_COMMANDShape for the datasource model. |
| `TestResponseToModel_COMMANDShape` | Unit | TestResponseToModel_COMMANDShape verifies responseToModel against the exact shape observed from a live initshutdownscript.create call in COMMAND mode: "script" comes back as an empty string, not null. |
| `TestResponseToModel_SCRIPTShape` | Unit | TestResponseToModel_SCRIPTShape mirrors TestResponseToModel_COMMANDShape for the SCRIPT type. |

## `internal/resources/ipmi_lan`  (30)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccIPMILanDataSource_basic` | Acceptance · HA gate | TestAccIPMILanDataSource_basic reads a channel's current LAN configuration through the truenas_ipmi_lan datasource only. |
| `TestAccIPMILan_setAndRestoreVlan` | Acceptance · HA gate | TestAccIPMILan_setAndRestoreVlan drives the truenas_ipmi_lan resource's "vlan" field through a different value, verifies it round-trips via Terraform (apply + import), then restores the channel's ENTIRE original config (dhcp/ip/netmask/gateway/vlan, not just vlan) via a direct API call registered in t.Cleanup before the mutating apply runs. |
| `TestIPMILanDataSourceModel_MatchesSchema` | Unit | TestIPMILanDataSourceModel_MatchesSchema is the datasource counterpart of TestIPMILanModel_MatchesSchema. |
| `TestIPMILanModel_MatchesSchema` | Unit | TestIPMILanModel_MatchesSchema verifies that every tfsdk tag on IPMILanModel has a corresponding attribute in the resource schema, and vice versa. |
| `TestIPMILanSchema_ApplyRemoteOptionalNotComputed` | Unit |  |
| `TestIPMILanSchema_ChannelRequiredForceNew` | Unit | TestIPMILanSchema_ChannelRequiredForceNew pins the identity contract from the task brief: "channel" identifies a pre-existing physical BMC LAN channel (ipmi.lan.update takes it as a positional argument), so it must be Required and force replacement rather than being renamed in place. |
| `TestIPMILanSchema_ComputedOnlyFields` | Unit |  |
| `TestIPMILanSchema_DHCPRequired` | Unit |  |
| `TestIPMILanSchema_IDIsComputed` | Unit |  |
| `TestIPMILanSchema_NoUnexpectedAttributes` | Unit |  |
| `TestIPMILanSchema_OptionalComputedFields` | Unit | TestIPMILanSchema_OptionalComputedFields pins the remaining Optional+Computed fields: settable, but always readable back from ipmi.lan.query. |
| `TestIPMILanSchema_PasswordWriteOnly` | Unit | TestIPMILanSchema_PasswordWriteOnly pins the password evidence from the live probe (ipmi.lan.query never returns a "password" key under any name): the schema field must be Sensitive + WriteOnly + Optional, never Computed (a WriteOnly attribute must not be Computed per the framework). |
| `TestIsDHCP` | Unit | --- isDHCP ------------------------------------------------------------ |
| `TestParseChannel` | Unit | --- parseChannel -------------------------------------------------------- |
| `TestResponseToDataSourceModel` | Unit | --- responseToDataSourceModel --------------------------------------------- |
| `TestResponseToModel` | Unit |  |
| `TestResponseToModel_DHCPSource` | Unit |  |
| `TestResponseToModel_PasswordUntouched` | Unit | responseToModel deliberately does not touch Password/ApplyRemote; callers must preserve whatever the model already had. |
| `TestResponseToModel_VlanSet` | Unit |  |
| `TestUpdatePayload_ApplyRemoteOnlySentWhenConfigured` | Unit |  |
| `TestUpdatePayload_DHCPFalseRequiresStaticFields` | Unit |  |
| `TestUpdatePayload_DHCPFalseWithStaticFields` | Unit |  |
| `TestUpdatePayload_DHCPTrueOmitsStaticFields` | Unit | --- updatePayload --------------------------------------------------------- |
| `TestUpdatePayload_PasswordOnlySentWhenConfiguredAndNonEmpty` | Unit |  |
| `TestUpdatePayload_VlanNeverSentAsExplicitNull` | Unit | TestUpdatePayload_VlanNeverSentAsExplicitNull covers every shape of "vlan unconfigured": updatePayload takes no state into account at all (an earlier version did — a "stateVlanSet" parameter — but that was reverted; see updatePayload's doc comment for the full reasoning and the live-confirmed crash it caused). |
| `TestUpdatePayload_VlanOnlySentWhenConfigured` | Unit |  |
| `TestVlanCannotBeCleared_StateNullConfigNull` | Unit | --- vlanCannotBeCleared ------------------------------------------------- |
| `TestVlanCannotBeCleared_StateNullConfigSet` | Unit |  |
| `TestVlanCannotBeCleared_StateSetConfigMatches` | Unit |  |
| `TestVlanCannotBeCleared_StateSetConfigNull` | Unit |  |

## `internal/resources/iscsi_auth`  (10)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccISCSIAuth_basic` | Acceptance · Tier 1 | TestAccISCSIAuth_basic tests create, update, and import of an iSCSI CHAP auth entry. |
| `TestISCSIAuthApiPayload_AlwaysIncludesRequired` | Unit | TestISCSIAuthApiPayload_AlwaysIncludesRequired verifies that tag/user/secret are always present in the payload, and that optional fields are omitted when null/unknown. |
| `TestISCSIAuthApiPayload_EmptyPeerSecretOmitted` | Unit | TestISCSIAuthApiPayload_EmptyPeerSecretOmitted verifies that a known but empty peersecret is NOT included in the payload (guards against clearing mutual CHAP on the server unintentionally / matches "only when non-empty"). |
| `TestISCSIAuthApiPayload_OptionalFieldsIncludedWhenKnown` | Unit | TestISCSIAuthApiPayload_OptionalFieldsIncludedWhenKnown verifies that peeruser/discovery_auth are included when known, and peersecret is included when known and non-empty. |
| `TestISCSIAuthApiPayload_UnknownOptionalFieldsOmitted` | Unit | TestISCSIAuthApiPayload_UnknownOptionalFieldsOmitted verifies that unknown (not yet resolved) optional fields are omitted from the payload. |
| `TestISCSIAuthDataSourceModel_MatchesSchema` | Unit | TestISCSIAuthDataSourceModel_MatchesSchema verifies that every tfsdk tag on ISCSIAuthDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestISCSIAuthDataSourceModel_NoSecretFields` | Unit | TestISCSIAuthDataSourceModel_NoSecretFields verifies (via reflection) that ISCSIAuthDataSourceModel has no "secret" or "peersecret" fields, so CHAP secrets can never be exposed through the datasource. |
| `TestISCSIAuthResponseToModel` | Unit | TestISCSIAuthResponseToModel verifies field mapping when the API returns non-empty values for everything. |
| `TestISCSIAuthResponseToModel_SecretsWriteOnly` | Unit | TestISCSIAuthResponseToModel_SecretsWriteOnly verifies that responseToModel never assigns to Secret/PeerSecret regardless of what the API returns: whatever the caller already had in the model (set or null) survives unchanged. |
| `TestISCSIAuthSchema` | Unit | TestISCSIAuthSchema verifies that the resource schema has the expected attributes, types, and Required/Optional/Computed/Sensitive flags. |

## `internal/resources/iscsi_extent`  (9)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccISCSIExtent_basic` | Acceptance · Tier 1 | TestAccISCSIExtent_basic creates a zvol fixture and a DISK-backed iSCSI extent on top of it, checks attributes, updates the comment in place, imports by id, and verifies destruction. |
| `TestISCSIExtentApiPayload_AvailThreshold` | Unit | TestISCSIExtentApiPayload_AvailThreshold verifies that avail_threshold=0 sends nil (pointer), and non-zero sends the value. |
| `TestISCSIExtentApiPayload_DISK` | Unit | TestISCSIExtentApiPayload_DISK verifies that DISK type payload contains the 'disk' key and not 'path'. |
| `TestISCSIExtentApiPayload_FILE` | Unit | TestISCSIExtentApiPayload_FILE verifies that FILE type payload contains the 'path' key and not 'disk'. |
| `TestISCSIExtentApiPayload_OmitsUnsetOptionalFields` | Unit | TestISCSIExtentApiPayload_OmitsUnsetOptionalFields verifies that when the Optional+Computed fields are null/unknown (the state a Create call sees for attributes the caller never set in config), apiPayload omits them entirely rather than sending Go zero values (0, "", false) that TrueNAS TrueNAS rejects for enum-constrained fields like blocksize and rpm. |
| `TestISCSIExtentResponseToModel_AllFields` | Unit | TestISCSIExtentResponseToModel_AllFields verifies that all fields are correctly mapped from the API response. |
| `TestISCSIExtentResponseToModel_NilAvailThreshold` | Unit | TestISCSIExtentResponseToModel_NilAvailThreshold verifies that a nil avail_threshold from the API maps to 0 in the model. |
| `TestISCSIExtentResponseToModel_NilDisk` | Unit | TestISCSIExtentResponseToModel_NilDisk verifies that a nil Disk pointer maps to an empty string in the model. |
| `TestISCSIExtentSchema` | Unit | TestISCSIExtentSchema verifies that the resource schema has the expected attributes and key attributes have the correct types. |

## `internal/resources/iscsi_global`  (14)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccISCSIGlobalDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccISCSIGlobalDataSource_basic reads the current TrueNAS iSCSI global configuration through the truenas_iscsi_global datasource only. |
| `TestAccISCSIGlobal_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccISCSIGlobal_setAndRestore drives the singleton truenas_iscsi_global resource's "pool_avail_threshold" field through a test value and back to the value read from the box before the test ran, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: ISCSIGlobalResource.Delete calls only this pure function. |
| `TestISCSIGlobalDataSourceModel_MatchesSchema` | Unit | TestISCSIGlobalDataSourceModel_MatchesSchema verifies that every tfsdk tag on ISCSIGlobalDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestISCSIGlobalSchema_AllFieldsOptionalComputed` | Unit | TestISCSIGlobalSchema_AllFieldsOptionalComputed verifies that every non-id field is Optional+Computed with UseStateForUnknown plan modifiers. |
| `TestISCSIGlobalSchema_IDIsComputed` | Unit | TestISCSIGlobalSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestResponseToModel_ISNSServersNilBecomesEmptyList` | Unit | TestResponseToModel_ISNSServersNilBecomesEmptyList verifies that a nil ISNSServers slice from the API maps to an empty (non-null) list in the model. |
| `TestResponseToModel_PoolAvailThresholdNilBecomesZero` | Unit | TestResponseToModel_PoolAvailThresholdNilBecomesZero verifies that a nil PoolAvailThreshold from the API maps to 0 in the model. |
| `TestResponseToModel_PoolAvailThresholdSet` | Unit | TestResponseToModel_PoolAvailThresholdSet verifies that a non-nil PoolAvailThreshold from the API is carried through as-is. |
| `TestUpdatePayload_AllKnownFieldsSent` | Unit | TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes every guarded field when its model value is known. |
| `TestUpdatePayload_ISNSServersNilElementsBecomeEmptySlice` | Unit | TestUpdatePayload_ISNSServersNilElementsBecomeEmptySlice verifies that a known but empty isns_servers list is sent as an empty slice, not nil/null. |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any field whose model value is null or unknown. |
| `TestUpdatePayload_PoolAvailThresholdNullOmitted` | Unit | TestUpdatePayload_PoolAvailThresholdNullOmitted verifies that a null/unknown threshold is omitted entirely from the payload. |
| `TestUpdatePayload_PoolAvailThresholdZeroSendsNil` | Unit | TestUpdatePayload_PoolAvailThresholdZeroSendsNil verifies that an explicitly-set 0 threshold is sent as a nil value in the payload (clearing the threshold on TrueNAS), not omitted and not sent as the literal 0. |

## `internal/resources/iscsi_initiator`  (6)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccISCSIInitiator_basic` | Acceptance · Tier 1 | TestAccISCSIInitiator_basic tests create, update, and import of an iSCSI initiator group. |
| `TestISCSIInitiatorApiPayload` | Unit | TestISCSIInitiatorApiPayload verifies that apiPayload produces a map with the expected keys and values. |
| `TestISCSIInitiatorApiPayload_NullInitiators` | Unit | TestISCSIInitiatorApiPayload_NullInitiators verifies that a null initiators list defaults to an empty (non-nil) slice in the payload. |
| `TestISCSIInitiatorResponseToModel` | Unit | TestISCSIInitiatorResponseToModel verifies that responseToModel populates all fields from the initiatorAPI struct correctly. |
| `TestISCSIInitiatorResponseToModel_NilInitiators` | Unit | TestISCSIInitiatorResponseToModel_NilInitiators verifies that a nil Initiators slice in the API response maps to an empty (non-nil) list. |
| `TestISCSIInitiatorSchema` | Unit | TestISCSIInitiatorSchema verifies that the resource schema has the expected attributes with correct types. |

## `internal/resources/iscsi_portal`  (8)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccISCSIPortal_basic` | Acceptance · Tier 1 | TestAccISCSIPortal_basic creates an iSCSI portal listening on the box IP (0.0.0.0 collides with the live portal on 26.0 — one portal per IP) (port is not settable per-listen on TrueNAS 26.0+; the global iSCSI listen_port applies), checks its attributes, updates its comment, imports it by id, and verifies destruction. |
| `TestISCSIPortalApiPayload_CommentGuarded` | Unit | TestISCSIPortalApiPayload_CommentGuarded verifies that comment is omitted from the payload when unset (null or unknown), and included when set. |
| `TestISCSIPortalApiPayload_ListenShape` | Unit | TestISCSIPortalApiPayload_ListenShape verifies that apiPayload produces the expected listen payload shape: ip always present, port NEVER present (TrueNAS 26.0 removed per-listen port from create/update; the global iSCSI listen_port governs), regardless of whether port is known or unknown in the model. |
| `TestISCSIPortalApiPayload_NilListen` | Unit | TestISCSIPortalApiPayload_NilListen verifies that a null listen list defaults to an empty slice (not nil) in the payload. |
| `TestISCSIPortalDataSourceModel_MatchesSchema` | Unit | TestISCSIPortalDataSourceModel_MatchesSchema verifies that every tfsdk tag on ISCSIPortalDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestISCSIPortalResponseToModel` | Unit | TestISCSIPortalResponseToModel verifies that responseToModel populates all fields correctly, including the nested listen list mapping. |
| `TestISCSIPortalResponseToModel_NilListen` | Unit | TestISCSIPortalResponseToModel_NilListen verifies that a nil Listen slice in the API response maps to an empty (not null) Terraform list. |
| `TestISCSIPortalSchema` | Unit | TestISCSIPortalSchema verifies that the resource schema has the expected attributes with the correct types. |

## `internal/resources/iscsi_target`  (5)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccISCSITarget_basic` | Acceptance · Tier 1 | TestAccISCSITarget_basic creates an own portal fixture (listening on 0.0.0.0, distinct from the box's live portal/target, id=1 "proxmox"; port is not settable per-listen on TrueNAS 26.0+), creates an iSCSI target referencing it, checks attributes, updates the alias in place, imports by id, and verifies destruction. |
| `TestISCSITargetApiPayload` | Unit | TestISCSITargetApiPayload verifies that apiPayload produces a map with the expected keys and correct values, including groups encoding. |
| `TestISCSITargetApiPayload_NullLists` | Unit | TestISCSITargetApiPayload_NullLists verifies that null groups/auth_networks default to empty slices (not nil). |
| `TestISCSITargetResponseToModel` | Unit | TestISCSITargetResponseToModel verifies that responseToModel populates all fields correctly, including nil pointer handling for alias/initiator/auth. |
| `TestISCSITargetSchema` | Unit | TestISCSITargetSchema verifies that the resource schema has the expected attributes with the correct types. |

## `internal/resources/iscsi_targetextent`  (7)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccISCSIEndToEnd` | Acceptance · Tier 1 | TestAccISCSIEndToEnd wires up a full, self-contained iSCSI configuration (portal, initiator, CHAP auth, a zvol fixture, an extent backed by that zvol, a target referencing the portal+initiator, and a targetextent association) entirely from own-created tf-acc objects. |
| `TestTargetExtentApiPayload_NullLunID` | Unit | TestTargetExtentApiPayload_NullLunID verifies that apiPayload omits lunid when it is null, so the API can auto-assign. |
| `TestTargetExtentApiPayload_UnknownLunID` | Unit | TestTargetExtentApiPayload_UnknownLunID verifies that apiPayload omits lunid when it is unknown (e.g. |
| `TestTargetExtentApiPayload_WithLunID` | Unit | TestTargetExtentApiPayload_WithLunID verifies that apiPayload includes lunid when it is known. |
| `TestTargetExtentDataSourceModel_MatchesSchema` | Unit | TestTargetExtentDataSourceModel_MatchesSchema verifies that every tfsdk tag on TargetExtentDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestTargetExtentResponseToModel` | Unit | TestTargetExtentResponseToModel verifies that responseToModel populates all fields from the targetExtentAPI struct correctly. |
| `TestTargetExtentSchema` | Unit | TestTargetExtentSchema verifies that the resource schema has the expected attributes with correct types and plan modifiers. |

## `internal/resources/kerberos_config`  (10)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccKerberosConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccKerberosConfigDataSource_basic reads the current TrueNAS Kerberos configuration through the truenas_kerberos_config datasource only. |
| `TestAccKerberosConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccKerberosConfig_setAndRestore drives the singleton truenas_kerberos_config resource's "appdefaults_aux" field through a marker value and back to the value read from the box before the test ran, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable. |
| `TestKerberosConfigDataSourceModel_MatchesSchema` | Unit | TestKerberosConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on KerberosConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestKerberosConfigSchema_AuxFieldsAreOptionalComputed` | Unit | TestKerberosConfigSchema_AuxFieldsAreOptionalComputed verifies that appdefaults_aux and libdefaults_aux are both Optional+Computed with UseStateForUnknown plan modifiers. |
| `TestKerberosConfigSchema_IDIsComputed` | Unit | TestKerberosConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestResponseToDataSourceModel` | Unit | TestResponseToDataSourceModel mirrors TestResponseToModel for the datasource model. |
| `TestResponseToModel` | Unit | TestResponseToModel verifies responseToModel against the shape probed from a live kerberos.config call. |
| `TestUpdatePayload_AllFieldsSet` | Unit | TestUpdatePayload_AllFieldsSet verifies that updatePayload includes every field with the exact keys observed in the kerberos.update probe. |
| `TestUpdatePayload_UnsetOptionalsOmitted` | Unit | TestUpdatePayload_UnsetOptionalsOmitted verifies that both fields are omitted when null/unknown, so the current TrueNAS-side value is left unchanged rather than overwritten with a zero value. |

## `internal/resources/kerberos_keytab`  (9)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccKerberosKeytab_basic` | Acceptance · Tier 1 | TestAccKerberosKeytab_basic creates a Kerberos keytab entry from a real keytab exported from a live Samba AD domain controller, checks its attributes (including that "file" round-trips intact into state — see model.go's kerberosKeytabAPI doc comment for the probe that established this), renames it in place, imports it by numeric id, and verifies destruction via a live kerberos.keytab.query. |
| `TestApiPayload_NoDoubleEncode` | Unit | TestApiPayload_NoDoubleEncode verifies that apiPayload passes the configured "file" value through byte-for-byte — it must never re-encode (e.g. |
| `TestApiPayload_PassesFieldsThrough` | Unit | TestApiPayload_PassesFieldsThrough verifies apiPayload includes exactly "name" and "file" with the exact keys observed in the kerberos.keytab.create/update probe, and no others. |
| `TestKerberosKeytabDataSourceModel_MatchesSchema` | Unit | TestKerberosKeytabDataSourceModel_MatchesSchema verifies that every tfsdk tag on KerberosKeytabDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestKerberosKeytabSchema_FileIsSensitiveNotWriteOnly` | Unit | TestKerberosKeytabSchema_FileIsSensitiveNotWriteOnly verifies that "file" is Required + Sensitive, and specifically NOT Computed and NOT WriteOnly — the probe in model.go's kerberosKeytabAPI doc comment found kerberos.keytab.query/get_instance return "file" intact (never redacted or omitted), so this resource uses the normal Sensitive modeling rather than the WriteOnly pattern (contrast with truenas_user's "password"). |
| `TestKerberosKeytabSchema_IDIsComputed` | Unit | TestKerberosKeytabSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestKerberosKeytabSchema_NameIsRequired` | Unit | TestKerberosKeytabSchema_NameIsRequired verifies that "name" is Required and carries no plan modifiers forcing replacement, since kerberos.keytab.update accepts an updated "name" (renamable in place). |
| `TestResponseToDataSourceModel` | Unit | TestResponseToDataSourceModel mirrors TestResponseToModel for the datasource model. |
| `TestResponseToModel` | Unit | TestResponseToModel verifies responseToModel against the exact shape observed from a live kerberos.keytab.create/get_instance/query call: id, name, and file all present, file byte-for-byte identical to what was sent (see model.go's kerberosKeytabAPI doc comment for the full probe writeup). |

## `internal/resources/kerberos_realm`  (12)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccKerberosRealm_basic` | Acceptance · Tier 1 | TestAccKerberosRealm_basic creates a synthetic Kerberos realm, checks its attributes, updates its admin_server list in place, imports it by its numeric id, and verifies destruction via a live kerberos.realm.query. |
| `TestApiPayload_AllFieldsSet` | Unit | TestApiPayload_AllFieldsSet verifies that apiPayload includes every field — realm, primary_kdc, kdc, admin_server, kpasswd_server — when all are known, with the exact keys observed in the kerberos.realm.create probe. |
| `TestApiPayload_EmptyListIsSentExplicitly` | Unit | TestApiPayload_EmptyListIsSentExplicitly verifies that a known, non-null empty list (e.g. |
| `TestApiPayload_UnsetOptionalsOmitted` | Unit | TestApiPayload_UnsetOptionalsOmitted verifies that primary_kdc, kdc, admin_server, and kpasswd_server are omitted from the payload when null or unknown, leaving only the Required "realm" field — so the TrueNAS-side defaults (primary_kdc=null, kdc=[], admin_server=[], kpasswd_server=[]) take effect instead of an explicit zero value being sent. |
| `TestKerberosRealmDataSourceModel_MatchesSchema` | Unit | TestKerberosRealmDataSourceModel_MatchesSchema verifies that every tfsdk tag on KerberosRealmDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestKerberosRealmSchema_IDIsComputed` | Unit | TestKerberosRealmSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestKerberosRealmSchema_OptionalComputedFields` | Unit | TestKerberosRealmSchema_OptionalComputedFields verifies primary_kdc, kdc, admin_server, and kpasswd_server are all Optional+Computed with UseStateForUnknown plan modifiers, matching the TrueNAS-side defaults observed in the kerberos.realm.create probe. |
| `TestKerberosRealmSchema_RealmIsRequired` | Unit | TestKerberosRealmSchema_RealmIsRequired verifies that "realm" is Required and carries no plan modifiers forcing replacement, since kerberos.realm.update accepts an updated "realm" value (renamable in place). |
| `TestResponseToDataSourceModel` | Unit | TestResponseToDataSourceModel mirrors TestResponseToModel for the datasource model. |
| `TestResponseToModel` | Unit | TestResponseToModel verifies responseToModel against the exact shape observed from a live kerberos.realm.get_instance/query call, including a non-null primary_kdc. |
| `TestResponseToModel_NilListsBecomeEmptyLists` | Unit | TestResponseToModel_NilListsBecomeEmptyLists verifies that nil kdc/ admin_server/kpasswd_server slices from the API map to empty (non-null) lists, matching the nil-guard convention used elsewhere for API-returned lists. |
| `TestResponseToModel_NilPrimaryKDCStaysNull` | Unit | TestResponseToModel_NilPrimaryKDCStaysNull verifies that a nil primary_kdc from the API maps to a null (not empty-string) types.String, preserving the distinction the anyOf string\|null API shape carries. |

## `internal/resources/keychain_ssh_connection`  (8)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccKeychainSSHConnection_loopback` | Acceptance · Tier 1 | TestAccKeychainSSHConnection_loopback exercises the full contract of truenas_keychain_ssh_connection against a loopback connection to the acceptance-test TrueNAS box itself: a fixture truenas_keychain_ssh_keypair (an ed25519 key generated in-test, so its public half is known ahead of apply time — unlike generate=true, whose key only exists after the API call), a throwaway truenas_user fixture (HCL-managed, own home dataset, destroyed along with everything else at the end of the test — see replication's TestAccReplication_RemoteSSH, whose fixture-user pattern this copies) with the generated public key authorized directly via its sshpubkey attribute, and the target host's key discovered via a live remote_ssh_host_key_scan. |
| `TestCreatePayload` | Unit | TestCreatePayload verifies createPayload sends the full attributes object plus name+type — every SSH_CREDENTIALS field always included, since keychaincredential.create's "attributes" is a single required object (probed live, no partial variant). |
| `TestKeychainSSHConnectionDataSourceModel_MatchesSchema` | Unit | TestKeychainSSHConnectionDataSourceModel_MatchesSchema verifies that every tfsdk tag on KeychainSSHConnectionDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestResponseToModel` | Unit | TestResponseToModel verifies responseToModel maps every API field onto the Terraform model, including renaming the wire's "private_key" (a keychain credential id, not key material) to PrivateKeyID. |
| `TestSchema_IDComputedOnly` | Unit | TestSchema_IDComputedOnly verifies "id" is Computed-only. |
| `TestSchema_OptionalComputedDefaultedFields` | Unit | TestSchema_OptionalComputedDefaultedFields verifies "port", "username", and "connect_timeout" are Optional+Computed — each has a server-side default (22, "root", 10 respectively, probed live) but is also updatable in place. |
| `TestSchema_RequiredFields` | Unit | TestSchema_RequiredFields verifies "name", "host", "private_key_id", and "remote_host_key" are Required, matching SSH_CREDENTIALS' own required set (probed live: host/private_key/remote_host_key), plus "name" which every keychaincredential entry requires. |
| `TestUpdatePayload` | Unit | TestUpdatePayload verifies updatePayload sends the same complete attributes object as createPayload (plus name, no "type") — keychaincredential.update requires the full attributes value, never a partial merge (probed live and per the API's own description). |

## `internal/resources/keychain_ssh_keypair`  (12)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccKeychainSSHKeyPair_generated` | Acceptance · Tier 1 | TestAccKeychainSSHKeyPair_generated exercises the generate=true path: TrueNAS generates the key pair server-side (via keychaincredential.generate_ssh_key_pair), rename in place, import by numeric id ("generate" is unknowable on import — see schema.go), destroy verified via a live keychaincredential.query. |
| `TestAccKeychainSSHKeyPair_supplied` | Acceptance · Tier 1 | TestAccKeychainSSHKeyPair_supplied exercises the user-supplied-key path: an ed25519 key pair generated in-test (Go stdlib crypto/ed25519 + golang.org/x/crypto/ssh's OpenSSH marshaling — see genEd25519OpenSSHKeyPair), verifying both that "private_key" round-trips byte-for-byte (it is Sensitive, not WriteOnly — probed live, see model.go) and that TrueNAS derives "public_key" automatically when it is omitted from the create payload. |
| `TestCreatePayload_GeneratedIncludesBothKeys` | Unit | TestCreatePayload_GeneratedIncludesBothKeys verifies the generated-key path (both private_key and public_key known from generate_ssh_key_pair's result) sends both fields. |
| `TestCreatePayload_SuppliedOmitsPublicKey` | Unit | TestCreatePayload_SuppliedOmitsPublicKey verifies the user-supplied-key path (empty publicKey — the caller only has the private key) omits "public_key" entirely, letting TrueNAS derive it server-side (probed live). |
| `TestKeychainSSHKeyPairDataSourceModel_MatchesSchema` | Unit | TestKeychainSSHKeyPairDataSourceModel_MatchesSchema verifies that every tfsdk tag on KeychainSSHKeyPairDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestResponseToModel_PreservesGenerate` | Unit | TestResponseToModel_PreservesGenerate verifies responseToModel never touches Generate (a provider-side-only field with no wire counterpart) — whatever the caller set beforehand survives unchanged. |
| `TestSchema_GenerateOptionalComputedRequiresReplace` | Unit | TestSchema_GenerateOptionalComputedRequiresReplace verifies "generate" is Optional+Computed (Create resolves an unset value to a concrete true/false) and forces replacement on change. |
| `TestSchema_IDComputedOnly` | Unit | TestSchema_IDComputedOnly verifies "id" is Computed-only. |
| `TestSchema_NameRequired` | Unit | TestSchema_NameRequired verifies "name" is Required (and not RequiresReplace — renaming happens in place). |
| `TestSchema_PrivateKeySensitiveComputedRequiresReplace` | Unit | TestSchema_PrivateKeySensitiveComputedRequiresReplace verifies "private_key" is Sensitive (but not WriteOnly — the probe showed keychaincredential.get_instance/query return it intact, unlike truenas_certificate's job-result masking), Optional+Computed (either user-supplied or server-generated, always read back), and immutable. |
| `TestSchema_PublicKeyComputedOnly` | Unit | TestSchema_PublicKeyComputedOnly verifies "public_key" is Computed-only — probed live, TrueNAS always derives it server-side and this resource never accepts it as input. |
| `TestUpdatePayload_NameOnly` | Unit | TestUpdatePayload_NameOnly verifies the update payload sends only "name" — private_key/public_key both carry RequiresReplace, so Update is never called across a change to either (probed live: a name-only update leaves the existing attributes untouched). |

## `internal/resources/lxc_config`  (20)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccLXCConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccLXCConfigDataSource_basic reads the current TrueNAS LXC configuration through the truenas_lxc_config datasource only. |
| `TestAccLXCConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccLXCConfig_setAndRestore drives the singleton truenas_lxc_config resource's "v4_network" field through one changed CIDR, then back to the value read from the box before the test ran, then imports it. |
| `TestBuildUpdatePayload_PreferredPoolBridgeSourcedFromPlanNotConfig` | Unit | TestBuildUpdatePayload_PreferredPoolBridgeSourcedFromPlanNotConfig verifies that preferred_pool/bridge are read from plan, NOT config: an explicit clear (plan null, e.g. |
| `TestBuildUpdatePayload_V4V6IncludedWhenExplicitlyConfigured` | Unit | TestBuildUpdatePayload_V4V6IncludedWhenExplicitlyConfigured verifies the config-driven guard does not suppress a value the user actually set. |
| `TestBuildUpdatePayload_V4V6SourcedFromConfigNotPlan` | Unit | TestBuildUpdatePayload_V4V6SourcedFromConfigNotPlan verifies that an unconfigured v4_network/v6_network (null in config) is omitted from the payload even when plan carries a stale non-null value forward via UseStateForUnknown -- the exact scenario that would silently resend a prior value on every apply if these two fields were sourced from plan. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies Delete's diagnostic builder returns exactly one warning and never touches a client. |
| `TestLXCConfigDataSourceModel_MatchesSchema` | Unit | TestLXCConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on LXCConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestLXCConfigSchema_AllSettableFieldsAreOptionalComputed` | Unit | TestLXCConfigSchema_AllSettableFieldsAreOptionalComputed verifies every field lxc.update actually accepts (per the probe) is Optional+Computed: preferred_pool, bridge (nullable three-way), v4_network, v6_network (plain guard). |
| `TestLXCConfigSchema_IDIsComputed` | Unit | TestLXCConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestLXCConfigSchema_NoUnexpectedAttributes` | Unit | TestLXCConfigSchema_NoUnexpectedAttributes verifies the schema exposes exactly the 5 attributes probed live (id + the 4 lxc.config/lxc.update fields) — nothing from the deprecated incus family (container.*) leaks in, since that namespace family is intentionally out of scope for this provider. |
| `TestResponseToDataSourceModel` | Unit | TestResponseToDataSourceModel verifies responseToDataSourceModel maps the same shape as responseToModel. |
| `TestResponseToModel` | Unit | TestResponseToModel verifies responseToModel against the shape probed from a live lxc.config call on TrueNAS 26.0. |
| `TestResponseToModel_NilPreferredPoolAndBridgeBecomeNull` | Unit | TestResponseToModel_NilPreferredPoolAndBridgeBecomeNull verifies a nil *string from the API (both probed live: lxc.config's preferred_pool and bridge are null on an unconfigured LXC install) maps to a null types.String, not an empty string. |
| `TestUpdatePayload_AllSet` | Unit | TestUpdatePayload_AllSet verifies updatePayload includes every field with the exact wire keys lxc.update accepts (probed live). |
| `TestUpdatePayload_BridgeThreeWay` | Unit | TestUpdatePayload_BridgeThreeWay verifies the three-way nullable convention for bridge, mirroring TestUpdatePayload_PreferredPoolThreeWay. |
| `TestUpdatePayload_PreferredPoolThreeWay` | Unit | TestUpdatePayload_PreferredPoolThreeWay verifies the three-way nullable convention for preferred_pool: unknown omits, null sends JSON nil (clears the pool), and a known value sends the string — mirroring docker_config's "pool" field. |
| `TestUpdatePayload_UnknownOmitted` | Unit | TestUpdatePayload_UnknownOmitted verifies unknown preferred_pool/bridge/ v4_network/v6_network are omitted entirely, leaving the current TrueNAS-side value unchanged rather than overwriting it. |
| `TestUpdatePayload_V4V6NetworkNullOmitted` | Unit | TestUpdatePayload_V4V6NetworkNullOmitted verifies v4_network/v6_network use a plain (not three-way) guard: null behaves the same as unknown (omitted), since lxc.config never returns null for either field. |
| `TestVersionGateDiagnostics_AtOrAboveFloor` | Unit | TestVersionGateDiagnostics_AtOrAboveFloor verifies no diagnostic is emitted for the exact 26.0 floor and releases above it, including the live-probed 26.0 beta string (see model.go's lxcConfigAPI doc comment). |
| `TestVersionGateDiagnostics_BelowFloor` | Unit | TestVersionGateDiagnostics_BelowFloor verifies the exact diagnostic emitted for a probed version below TrueNAS 26.0, including the 25.10 string actually observed live (see model.go's lxcConfigAPI doc comment). |

## `internal/resources/mail`  (17)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccMailDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccMailDataSource_basic reads the current TrueNAS mail configuration through the truenas_mail datasource only. |
| `TestAccMail_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccMail_setAndRestore drives the singleton truenas_mail resource's "fromname" field (a cosmetic, low-risk display name) through a test value and back to the value read from the box before the test ran, then imports it. |
| `TestBasePayloadFromConfig_IncludesAllWritableFields` | Unit | TestBasePayloadFromConfig_IncludesAllWritableFields verifies that basePayloadFromConfig carries every writable, non-secret field from a live mailAPI response into the base payload, including a nil User mapping to a nil (not omitted) "user" entry. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: MailResource.Delete calls only this pure function. |
| `TestMailSchema_IDIsComputed` | Unit | TestMailSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute, since it's a fixed singleton value never supplied by the user. |
| `TestMailSchema_OtherFieldsAreOptionalComputed` | Unit | TestMailSchema_OtherFieldsAreOptionalComputed verifies that every non-pass, non-id config field is Optional+Computed with UseStateForUnknown plan modifiers, matching the "all other fields" contract in the task brief. |
| `TestMailSchema_PassIsSensitiveWriteOnly` | Unit | TestMailSchema_PassIsSensitiveWriteOnly verifies that "pass" is Optional + Sensitive, and specifically NOT Computed (write-only: never read back from TrueNAS, so it must not participate in drift detection). |
| `TestMergedPayload_PlanOverlaysLive` | Unit | TestMergedPayload_PlanOverlaysLive verifies that mergedPayload starts from the live config's fields and that any field the plan knows about overrides the live value, while fields the plan doesn't know about keep their live value — this is the fix for "mail_update.fromemail: This field is required" when a config sets only fromname. |
| `TestMergedPayload_SecretAbsentUnlessSet` | Unit | TestMergedPayload_SecretAbsentUnlessSet verifies that mergedPayload never includes "pass" unless the plan explicitly sets it: the base built from the live config has no secret fields, and updatePayload only contributes pass when it's known and non-null. |
| `TestResponseToDataSourceModel_UserNilBecomesEmptyString` | Unit | TestResponseToDataSourceModel_UserNilBecomesEmptyString mirrors the same nil-to-empty-string handling for the datasource model. |
| `TestResponseToModel_PassNeverSet` | Unit | TestResponseToModel_PassNeverSet verifies that responseToModel never writes to m.Pass, regardless of its prior value: pass is write-only and the API never returns it. |
| `TestResponseToModel_UserNilBecomesEmptyString` | Unit | TestResponseToModel_UserNilBecomesEmptyString verifies that a nil User from the API maps to an empty string in the model (not null). |
| `TestResponseToModel_UserSet` | Unit | TestResponseToModel_UserSet verifies that a non-nil User from the API is carried through as-is. |
| `TestUpdatePayload_AllKnownFieldsSent` | Unit | TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes every guarded field when its model value is known. |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any field whose model value is null or unknown, for every guarded field except user/pass (which have their own tests below). |
| `TestUpdatePayload_PassOnlyWhenSet` | Unit | TestUpdatePayload_PassOnlyWhenSet verifies that pass is included only when it has a known, non-null value, and is otherwise omitted entirely (never sent as an empty string or null). |
| `TestUpdatePayload_UserOnlyWhenNonEmpty` | Unit | TestUpdatePayload_UserOnlyWhenNonEmpty verifies that user is omitted when null/unknown, sent as nil when explicitly cleared to an empty string (so a previously set user can be cleared), and sent as-is when non-empty. |

## `internal/resources/network_config`  (15)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNetworkConfigDataSource_basic` | Acceptance · conditional skip | TestAccNetworkConfigDataSource_basic reads the current TrueNAS global network configuration through the truenas_network_config datasource only. |
| `TestAccNetworkConfig_basic` | Acceptance · conditional skip | TestAccNetworkConfig_basic is intentionally skipped by default. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: NetworkConfigResource.Delete calls only this pure function. |
| `TestNetworkConfigDataSourceModel_MatchesSchema` | Unit | TestNetworkConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on NetworkConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestNetworkConfigSchema_IDIsComputed` | Unit | TestNetworkConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestNetworkConfigSchema_WritableFieldsOptionalComputed` | Unit | TestNetworkConfigSchema_WritableFieldsOptionalComputed verifies that every writable field is Optional+Computed with a UseStateForUnknown plan modifier. |
| `TestResponseToModel_ListsNilMapToEmpty` | Unit | TestResponseToModel_ListsNilMapToEmpty verifies that nil domains/hosts from the API map to empty (non-null) Terraform lists. |
| `TestResponseToModel_MapsPlainFields` | Unit | TestResponseToModel_MapsPlainFields verifies that non-nullable, non-pointer fields are copied through unchanged. |
| `TestResponseToModel_NullableStringFieldsMapToEmpty` | Unit | TestResponseToModel_NullableStringFieldsMapToEmpty verifies that a nil ipv4gateway/ipv6gateway/nameserver1-3 on the wire maps to "" (not a Terraform null), and that a non-nil value (including an explicit "" from a live system) maps through unchanged. |
| `TestResponseToModel_ServiceAnnouncementMapping` | Unit | TestResponseToModel_ServiceAnnouncementMapping verifies that a present service_announcement object on the wire maps to a known (non-null) Terraform object with the correct field values, and that a nil service_announcement maps to an ObjectNull. |
| `TestUpdatePayload_AllKnownFieldsSent` | Unit | TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes every guarded field when its model value is known. |
| `TestUpdatePayload_DomainsHostsNilToEmpty` | Unit | TestUpdatePayload_DomainsHostsNilToEmpty verifies that a known-but-nil ElementsAs result for domains/hosts is normalized to an empty slice (rather than sending a Go nil, which would marshal to JSON null instead of an empty array). |
| `TestUpdatePayload_GatewayNameserverThreeWay` | Unit | TestUpdatePayload_GatewayNameserverThreeWay verifies the three-way behavior shared by ipv4gateway, ipv6gateway, and nameserver1-3: null and unknown omit the key entirely, an explicit "" sends JSON nil (clearing the value), and any other value sends that value. |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any field whose model value is null or unknown. |
| `TestUpdatePayload_ServiceAnnouncementOnlyWhenKnown` | Unit | TestUpdatePayload_ServiceAnnouncementOnlyWhenKnown verifies that service_announcement is omitted entirely when null/unknown, and sent as a full 3-key map only when known. |

## `internal/resources/network_interface`  (17)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNetworkInterface_datasourceEnp7s0` | Acceptance · Tier 1 | TestAccNetworkInterface_datasourceEnp7s0 is a strictly read-only datasource lookup of the box's physical management NIC (enp7s0). |
| `TestAccNetworkInterface_bridge` | Acceptance · conditional skip | TestAccNetworkInterface_bridge would create a BRIDGE interface (br999) with no members and no aliases, verify it, then destroy it. |
| `TestCreatePayload_AliasEmptyTypeOmitted` | Unit | TestCreatePayload_AliasEmptyTypeOmitted verifies that an alias whose type is unset (empty string) does not send "type": "" in the payload -- TrueNAS infers INET/INET6 from the address itself. |
| `TestCreatePayload_Bridge` | Unit | TestCreatePayload_Bridge verifies that a BRIDGE create payload contains bridge_members (and stp) but no lag/vlan keys, plus name+type. |
| `TestCreatePayload_HasNameAndType` | Unit | TestCreatePayload_HasNameAndType verifies the create payload always carries name and type. |
| `TestCreatePayload_LagFieldsUnsetOmitted` | Unit | TestCreatePayload_LagFieldsUnsetOmitted verifies that lag_protocol is omitted from the payload when null/unknown, rather than sent as "" (the Go zero value for types.String), which would incorrectly overwrite the API's chosen default. |
| `TestCreatePayload_LinkAggregation` | Unit | TestCreatePayload_LinkAggregation verifies that a LINK_AGGREGATION create payload contains lag_protocol/lag_ports but no bridge/vlan keys. |
| `TestCreatePayload_Vlan` | Unit | TestCreatePayload_Vlan verifies that a VLAN create payload contains vlan_parent_interface/vlan_tag but no bridge/lag keys. |
| `TestCreatePayload_VlanFieldsUnsetOmitted` | Unit | TestCreatePayload_VlanFieldsUnsetOmitted verifies that vlan_parent_interface and vlan_tag are omitted from the payload when null/unknown, rather than sent as ""/0. |
| `TestPayload_MTUNonZeroIncluded` | Unit | TestPayload_MTUNonZeroIncluded verifies that a non-zero mtu is included. |
| `TestPayload_MTUZeroOmitted` | Unit | TestPayload_MTUZeroOmitted verifies that mtu is omitted from the payload when set to 0 (unset). |
| `TestResponseToModel_Aliases` | Unit | TestResponseToModel_Aliases verifies aliases round-trip through the model. |
| `TestResponseToModel_NilPointers` | Unit | TestResponseToModel_NilPointers verifies that nil pointer fields in interfaceAPI map to zero values (mtu -> 0, stp -> false, lag_protocol -> "", vlan_parent_interface -> "", vlan_tag -> 0) as would happen for a PHYSICAL interface's query response. |
| `TestResponseToModel_NonNilPointers` | Unit | TestResponseToModel_NonNilPointers verifies that non-nil pointer fields are dereferenced correctly. |
| `TestUpdatePayload_NoNameOrType` | Unit | TestUpdatePayload_NoNameOrType verifies the update payload never carries name or type, unlike the create payload. |
| `TestValidateCreateType_PhysicalRejected` | Unit | TestValidateCreateType_PhysicalRejected verifies that creating a PHYSICAL interface is rejected. |
| `TestValidateCreateType_VirtualTypesAllowed` | Unit | TestValidateCreateType_VirtualTypesAllowed verifies that BRIDGE, LINK_AGGREGATION, and VLAN are all accepted for create. |

## `internal/resources/nfs`  (1)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNFSShare_basic` | Acceptance · Tier 1 | TestAccNFSShare_basic creates a dataset fixture, shares its mountpoint over NFS, checks attributes, updates a mutable field (comment), imports the share by its integer ID, and verifies destruction. |

## `internal/resources/nfs_config`  (20)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNFSConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccNFSConfigDataSource_basic reads the current TrueNAS NFS configuration through the truenas_nfs_config datasource only. |
| `TestAccNFSConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccNFSConfig_setAndRestore drives the singleton truenas_nfs_config resource's "v4_domain" field (a plain string, not part of the nullable-three-way group) through a test value and back to the value read from the box before the test ran, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: NFSConfigResource.Delete calls only this pure function. |
| `TestModifyPlan_CreateNoOp` | Unit | TestModifyPlan_CreateNoOp verifies that ModifyPlan does nothing when the state is null, i.e. |
| `TestModifyPlan_DestroyNoOp` | Unit | TestModifyPlan_DestroyNoOp verifies that ModifyPlan does nothing (no error, no attribute changes attempted) when the plan is null, i.e. |
| `TestModifyPlan_UpdateMarksServerMutableFieldsUnknown` | Unit | TestModifyPlan_UpdateMarksServerMutableFieldsUnknown verifies the fix for "managed_nfsd: was cty.True, but now cty.False": on an update (both plan and state non-null), ModifyPlan must mark managed_nfsd, v4_krb_enabled, and keytab_has_nfs_spn unknown, so a server-side flip of one of these booleans (e.g. |
| `TestNFSConfigDataSourceModel_MatchesSchema` | Unit | TestNFSConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on NFSConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestNFSConfigSchema_AllOtherFieldsOptionalComputed` | Unit | TestNFSConfigSchema_AllOtherFieldsOptionalComputed verifies that every writable field is Optional+Computed with a plan modifier, matching the "all writable fields Optional+Computed + UseStateForUnknown" contract in the task brief. |
| `TestNFSConfigSchema_ComputedOnlyTrioNeverOptional` | Unit | TestNFSConfigSchema_ComputedOnlyTrioNeverOptional verifies that managed_nfsd, v4_krb_enabled, and keytab_has_nfs_spn are Computed-only (never Optional) and carry UseStateForUnknown plan modifiers. |
| `TestNFSConfigSchema_IDIsComputed` | Unit | TestNFSConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestResponseToDataSourceModel_ListsNilBecomeEmptyList` | Unit | TestResponseToDataSourceModel_ListsNilBecomeEmptyList mirrors the same nil-to-empty-list handling for the datasource model. |
| `TestResponseToModel_ListsNilBecomeEmptyList` | Unit | TestResponseToModel_ListsNilBecomeEmptyList verifies that nil list fields from the API map to empty (non-null) lists in the model, for both list fields. |
| `TestResponseToModel_ListsSet` | Unit | TestResponseToModel_ListsSet verifies that non-nil list fields from the API are carried through as-is. |
| `TestResponseToModel_NullableIntsAPINilBecomesZero` | Unit | TestResponseToModel_NullableIntsAPINilBecomesZero verifies that a nil API pointer for each of the four nullable int fields maps to 0 in the model, for both the resource and datasource models. |
| `TestResponseToModel_NullableIntsSet` | Unit | TestResponseToModel_NullableIntsSet verifies that non-nil API values for each of the four nullable int fields are carried through as-is. |
| `TestUpdatePayload_AllKnownFieldsSent` | Unit | TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes every guarded writable field when its model value is known, and that the computed-only trio is excluded even though the model carries known values for them. |
| `TestUpdatePayload_ComputedOnlyTrioNeverInPayload` | Unit | TestUpdatePayload_ComputedOnlyTrioNeverInPayload verifies that managed_nfsd, v4_krb_enabled, and keytab_has_nfs_spn are never included in the update payload, even though the model carries known values for them: they are Computed-only and TrueNAS does not accept them as nfs.update arguments. |
| `TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice` | Unit | TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice verifies that a known but empty list is sent as an empty slice, not nil/null, for both list fields. |
| `TestUpdatePayload_NullableIntThreeWay` | Unit | TestUpdatePayload_NullableIntThreeWay verifies the three-way nullable handling for each of servers, mountd_port, rpcstatd_port, and rpclockd_port: omitted when null/unknown, sent as nil when explicitly set to 0, and sent as the value otherwise. |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any field whose model value is null or unknown. |

## `internal/resources/ntp_server`  (7)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNTPServer_basic` | Acceptance · Tier 1 | TestAccNTPServer_basic tests create, update, and import of an NTP server. |
| `TestNTPServerApiPayload_AllSet` | Unit | TestNTPServerApiPayload_AllSet verifies that apiPayload includes all optional fields, including force, when they are set. |
| `TestNTPServerApiPayload_ForceOmittedWhenUnset` | Unit | TestNTPServerApiPayload_ForceOmittedWhenUnset explicitly verifies force is omitted from the payload when unset, and included only when explicitly set. |
| `TestNTPServerApiPayload_OnlyAddress` | Unit | TestNTPServerApiPayload_OnlyAddress verifies that apiPayload contains only "address" when all optional fields (including force) are unset (null). |
| `TestNTPServerDataSourceModel_MatchesSchema` | Unit | TestNTPServerDataSourceModel_MatchesSchema verifies that every tfsdk tag on NTPServerDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestNTPServerResponseToModel` | Unit | TestNTPServerResponseToModel verifies that responseToModel populates all fields from the ntpServerAPI struct correctly, and never sets Force (which has no corresponding field in the API response). |
| `TestNTPServerSchema` | Unit | TestNTPServerSchema verifies that the resource schema has the expected attributes with correct types. |

## `internal/resources/nvmet_global`  (9)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNVMeTGlobalDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccNVMeTGlobalDataSource_basic reads the current TrueNAS NVMe-oF global configuration through the truenas_nvmet_global datasource only. |
| `TestAccNVMeTGlobal_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccNVMeTGlobal_setAndRestore drives the singleton truenas_nvmet_global resource's "xport_referral" field through its opposite value and back to the value read from the box before the test ran, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: NVMeTGlobalResource.Delete calls only this pure function. |
| `TestNVMeTGlobalDataSourceModel_MatchesSchema` | Unit | TestNVMeTGlobalDataSourceModel_MatchesSchema verifies that every tfsdk tag on NVMeTGlobalDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestNVMeTGlobalSchema_AllFieldsOptionalComputed` | Unit | TestNVMeTGlobalSchema_AllFieldsOptionalComputed verifies that every non-id field is Optional+Computed with UseStateForUnknown plan modifiers. |
| `TestNVMeTGlobalSchema_IDIsComputed` | Unit | TestNVMeTGlobalSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestResponseToModel_MapsAllFields` | Unit | TestResponseToModel_MapsAllFields verifies that responseToModel copies every API field onto the Terraform model, and sets the fixed singleton ID. |
| `TestUpdatePayload_AllKnownFieldsSent` | Unit | TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes every guarded field when its model value is known. |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any field whose model value is null or unknown. |

## `internal/resources/nvmet_host`  (13)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNVMetHost_basic` | Acceptance · Tier 1 | TestAccNVMetHost_basic tests create, update, and import of an NVMe-oF host (initiator). |
| `TestNVMetHostCreatePayload_AlwaysIncludesHostNQN` | Unit | TestNVMetHostCreatePayload_AlwaysIncludesHostNQN verifies that hostnqn is always present in the create payload, and that optional/secret fields are omitted when null. |
| `TestNVMetHostCreatePayload_EmptyDHGroupOmitted` | Unit | TestNVMetHostCreatePayload_EmptyDHGroupOmitted verifies that a known but empty dhchap_dhgroup is NOT included in the payload. |
| `TestNVMetHostCreatePayload_EmptySecretsOmitted` | Unit | TestNVMetHostCreatePayload_EmptySecretsOmitted verifies that known but empty dhchap_key/dhchap_ctrl_key are NOT included in the payload (guards against clearing/rewriting an existing secret unintentionally). |
| `TestNVMetHostCreatePayload_OptionalFieldsIncludedWhenKnown` | Unit | TestNVMetHostCreatePayload_OptionalFieldsIncludedWhenKnown verifies that description/dhchap_dhgroup/dhchap_hash and the secrets are included when known and non-empty. |
| `TestNVMetHostCreatePayload_UnknownOptionalFieldsOmitted` | Unit | TestNVMetHostCreatePayload_UnknownOptionalFieldsOmitted verifies that unknown (not yet resolved) optional fields are omitted from the payload. |
| `TestNVMetHostDataSourceModel_MatchesSchema` | Unit | TestNVMetHostDataSourceModel_MatchesSchema verifies that every tfsdk tag on NVMetHostDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestNVMetHostDataSourceModel_NoSecretFields` | Unit | TestNVMetHostDataSourceModel_NoSecretFields verifies (via reflection) that NVMetHostDataSourceModel has no "dhchap_key" or "dhchap_ctrl_key" fields, so DH-CHAP secrets can never be exposed through the datasource. |
| `TestNVMetHostResponseToModel` | Unit | TestNVMetHostResponseToModel verifies field mapping when the API returns non-empty values for everything. |
| `TestNVMetHostResponseToModel_NilDHGroupMapsToEmpty` | Unit | TestNVMetHostResponseToModel_NilDHGroupMapsToEmpty verifies that a nil dhchap_dhgroup from the API maps to an empty string rather than leaving the model attribute null/unknown. |
| `TestNVMetHostResponseToModel_SecretsWriteOnly` | Unit | TestNVMetHostResponseToModel_SecretsWriteOnly verifies that responseToModel never assigns to DHChapKey/DHChapCtrlKey regardless of what the caller already had in the model: set values are left unchanged, and null values stay null (never coerced to ""). |
| `TestNVMetHostSchema` | Unit | TestNVMetHostSchema verifies that the resource schema has the expected attributes, types, and Required/Optional/Computed/Sensitive flags. |
| `TestNVMetHostUpdatePayload_AlwaysIncludesHostNQN` | Unit | TestNVMetHostUpdatePayload_AlwaysIncludesHostNQN verifies that hostnqn is included in the update payload (it is updatable per the API and Required, so it is always known). |

## `internal/resources/nvmet_host_subsys`  (13)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNVMetHostSubsys_basic` | Acceptance · Tier 1 | TestAccNVMetHostSubsys_basic tests create and import of an NVMe-oF host/subsystem association. |
| `TestDecodeEmbeddedID_BareInt` | Unit | TestDecodeEmbeddedID_BareInt verifies decoding a bare integer shape, as used by create/update payload echoes. |
| `TestDecodeEmbeddedID_Null` | Unit | TestDecodeEmbeddedID_Null verifies that a null field returns an error rather than silently defaulting to 0. |
| `TestDecodeEmbeddedID_Object` | Unit | TestDecodeEmbeddedID_Object verifies decoding an embedded object shape ({"id": N, ...}), as returned by query/get_instance. |
| `TestDecodeEmbeddedID_ObjectNullID` | Unit | TestDecodeEmbeddedID_ObjectNullID verifies that an object shape whose "id" key is explicitly null returns an error rather than silently defaulting to 0. |
| `TestDecodeEmbeddedID_ObjectWithoutID` | Unit | TestDecodeEmbeddedID_ObjectWithoutID verifies that an object shape missing the "id" key returns an error rather than silently defaulting to 0. |
| `TestHostSubsysApiPayload` | Unit | TestHostSubsysApiPayload verifies that apiPayload includes exactly host_id and subsys_id. |
| `TestHostSubsysDataSourceModel_MatchesSchema` | Unit | TestHostSubsysDataSourceModel_MatchesSchema verifies that every tfsdk tag on HostSubsysDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestHostSubsysResponseToModel` | Unit | TestHostSubsysResponseToModel verifies that responseToModel decodes both embedded host and subsys objects and populates the model correctly. |
| `TestHostSubsysResponseToModel_BareIntShapes` | Unit | TestHostSubsysResponseToModel_BareIntShapes verifies responseToModel also handles bare-integer host/subsys shapes. |
| `TestHostSubsysResponseToModel_NullHost` | Unit | TestHostSubsysResponseToModel_NullHost verifies that a null host field surfaces as a diagnostic error rather than silently zeroing HostID. |
| `TestHostSubsysResponseToModel_NullSubsys` | Unit | TestHostSubsysResponseToModel_NullSubsys verifies that a null subsys field surfaces as a diagnostic error rather than silently zeroing SubsysID. |
| `TestHostSubsysSchema` | Unit | TestHostSubsysSchema verifies that the resource schema has the expected attributes with correct types and plan modifiers. |

## `internal/resources/nvmet_namespace`  (12)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNVMetNamespace_basic` | Acceptance · Tier 1 | TestAccNVMetNamespace_basic tests create, update, and import of an NVMe-oF namespace. |
| `TestCreatePayload_AllFieldsKnown` | Unit | TestCreatePayload_AllFieldsKnown verifies that every guarded field, including nsid, is present when all model values are known. |
| `TestCreatePayload_KeysAlwaysPresent` | Unit | TestCreatePayload_KeysAlwaysPresent verifies that "subsys_id" and "device_path" are always sent on create, and that "nsid" is omitted entirely when unset (auto-assign), while other guarded fields are also omitted when null/unknown. |
| `TestDecodeSubsysID_BareInt` | Unit | TestDecodeSubsysID_BareInt verifies decoding of a bare integer ID shape, as used in create/update payloads and possibly some responses. |
| `TestDecodeSubsysID_EmbeddedObject` | Unit | TestDecodeSubsysID_EmbeddedObject verifies decoding of the embedded subsys object shape returned by query/get_instance. |
| `TestDecodeSubsysID_Null` | Unit | TestDecodeSubsysID_Null verifies that a null subsys field returns an error rather than silently defaulting to 0. |
| `TestNVMetNamespaceDataSourceModel_MatchesSchema` | Unit | TestNVMetNamespaceDataSourceModel_MatchesSchema verifies that every tfsdk tag on NVMetNamespaceDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestNVMetNamespaceSchema` | Unit | TestNVMetNamespaceSchema verifies key schema attribute shapes. |
| `TestResponseToModel_AllFields` | Unit | TestResponseToModel_AllFields verifies that fields are correctly mapped from the API response, matching the live query shape from the box. |
| `TestResponseToModel_FilesizeNil` | Unit | TestResponseToModel_FilesizeNil verifies that a nil filesize from the API (typical for ZVOL-backed namespaces) maps to null rather than 0, so that a subsequent update payload built from this model omits "filesize" instead of sending an explicit 0 that would overwrite a server-side null. |
| `TestResponseToModel_InvalidSubsys` | Unit | TestResponseToModel_InvalidSubsys verifies that a malformed subsys field surfaces as a diagnostic error rather than silently defaulting. |
| `TestUpdatePayload_NoSubsysIDOrNSID` | Unit | TestUpdatePayload_NoSubsysIDOrNSID verifies that "subsys_id" and "nsid" are never included in the update payload, even when the model's SubsysID and NSID fields are known. |

## `internal/resources/nvmet_port`  (8)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNVMetPort_basic` | Acceptance · Tier 1 | TestAccNVMetPort_basic tests create, update, and import of an NVMe-oF port. |
| `TestCreatePayload_AllFieldsKnown` | Unit | TestCreatePayload_AllFieldsKnown verifies that every guarded field is present when all model values are known. |
| `TestCreatePayload_TrtypeAndTraddrAlwaysPresent` | Unit | TestCreatePayload_TrtypeAndTraddrAlwaysPresent verifies that "addr_trtype" and "addr_traddr" are always sent on create, even when every optional field is null/unknown. |
| `TestNVMetPortDataSourceModel_MatchesSchema` | Unit | TestNVMetPortDataSourceModel_MatchesSchema verifies that every tfsdk tag on NVMetPortDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestNVMetPortSchema` | Unit | TestNVMetPortSchema verifies key schema attribute shapes. |
| `TestResponseToModel_AllFields` | Unit | TestResponseToModel_AllFields verifies that non-nil fields are correctly mapped from the API response, matching the live query shape from the box. |
| `TestResponseToModel_NilPointers` | Unit | TestResponseToModel_NilPointers verifies that nil inline_data_size/ max_queue_size/pi_enable fields from the API map to null rather than a zero value, so that a subsequent update payload built from this model omits them (see guardedFields) instead of sending an explicit 0/false that would overwrite a server-side null. |
| `TestUpdatePayload_NoAddrTrtypeKey` | Unit | TestUpdatePayload_NoAddrTrtypeKey verifies that "addr_trtype" is never included in the update payload, even when the model's AddrTrtype field is known. |

## `internal/resources/nvmet_port_subsys`  (13)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNVMeTEndToEnd` | Acceptance · Tier 1 | TestAccNVMeTEndToEnd wires up a full, self-contained NVMe-oF configuration (subsystem, port, a zvol fixture, a namespace backed by that zvol, a host, a host/subsystem association, and a port/subsystem association) entirely from own-created tf-acc objects. |
| `TestDecodeEmbeddedID_BareInt` | Unit | TestDecodeEmbeddedID_BareInt verifies decoding a bare integer shape, as used by create/update payload echoes. |
| `TestDecodeEmbeddedID_Null` | Unit | TestDecodeEmbeddedID_Null verifies that a null field returns an error rather than silently defaulting to 0. |
| `TestDecodeEmbeddedID_Object` | Unit | TestDecodeEmbeddedID_Object verifies decoding an embedded object shape ({"id": N, ...}), as returned by query/get_instance. |
| `TestDecodeEmbeddedID_ObjectNullID` | Unit | TestDecodeEmbeddedID_ObjectNullID verifies that an object shape whose "id" key is explicitly null returns an error rather than silently defaulting to 0. |
| `TestDecodeEmbeddedID_ObjectWithoutID` | Unit | TestDecodeEmbeddedID_ObjectWithoutID verifies that an object shape missing the "id" key returns an error rather than silently defaulting to 0. |
| `TestPortSubsysApiPayload` | Unit | TestPortSubsysApiPayload verifies that apiPayload includes exactly port_id and subsys_id. |
| `TestPortSubsysDataSourceModel_MatchesSchema` | Unit | TestPortSubsysDataSourceModel_MatchesSchema verifies that every tfsdk tag on PortSubsysDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestPortSubsysResponseToModel` | Unit | TestPortSubsysResponseToModel verifies that responseToModel decodes both embedded port and subsys objects and populates the model correctly. |
| `TestPortSubsysResponseToModel_BareIntShapes` | Unit | TestPortSubsysResponseToModel_BareIntShapes verifies responseToModel also handles bare-integer port/subsys shapes. |
| `TestPortSubsysResponseToModel_NullPort` | Unit | TestPortSubsysResponseToModel_NullPort verifies that a null port field surfaces as a diagnostic error rather than silently zeroing PortID. |
| `TestPortSubsysResponseToModel_NullSubsys` | Unit | TestPortSubsysResponseToModel_NullSubsys verifies that a null subsys field surfaces as a diagnostic error rather than silently zeroing SubsysID. |
| `TestPortSubsysSchema` | Unit | TestPortSubsysSchema verifies that the resource schema has the expected attributes with correct types and plan modifiers. |

## `internal/resources/nvmet_subsys`  (10)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccNVMetSubsys_basic` | Acceptance · Tier 1 | TestAccNVMetSubsys_basic tests create, update, and import of an NVMe-oF subsystem. |
| `TestCreatePayload_AllFieldsKnown` | Unit | TestCreatePayload_AllFieldsKnown verifies that every guarded field is present when all model values are known and non-empty. |
| `TestCreatePayload_EmptySubNQNSendsNull` | Unit | TestCreatePayload_EmptySubNQNSendsNull verifies the three-way rule for subnqn/ieee_oui: null/unknown is omitted, but an explicitly known empty string is sent as an explicit JSON null (clear), distinct from omission. |
| `TestCreatePayload_NameAlwaysPresent` | Unit | TestCreatePayload_NameAlwaysPresent verifies that "name" is always sent on create, even when every optional field is null/unknown. |
| `TestCreatePayload_NullSubNQNOmitted` | Unit | TestCreatePayload_NullSubNQNOmitted verifies that a null/unknown subnqn/ ieee_oui (never set in config) is omitted entirely, distinct from an explicit empty string which sends a JSON null. |
| `TestNVMetSubsysDataSourceModel_MatchesSchema` | Unit | TestNVMetSubsysDataSourceModel_MatchesSchema verifies that every tfsdk tag on NVMetSubsysDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestNVMetSubsysSchema` | Unit | TestNVMetSubsysSchema verifies key schema attribute shapes. |
| `TestResponseToModel_AllFields` | Unit | TestResponseToModel_AllFields verifies that non-nil fields are correctly mapped from the API response, matching the live query shape from the box. |
| `TestResponseToModel_NilPointers` | Unit | TestResponseToModel_NilPointers verifies that nil ana/pi_enable/qid_max fields from the API map to null, while ieee_oui maps to empty string. |
| `TestUpdatePayload_NoNameKey` | Unit | TestUpdatePayload_NoNameKey verifies that "name" is never included in the update payload, even when the model's Name field is known. |

## `internal/resources/periodic_snapshot`  (3)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccPeriodicSnapshot_basic` | Acceptance · Tier 1 | TestAccPeriodicSnapshot_basic creates a dataset fixture and a periodic snapshot task on it, checks its attributes, updates the schedule minute and lifetime_value in place, imports the task by its numeric id, and verifies destruction of both the task and the dataset fixture. |
| `TestPeriodicSnapshotPayload` | Unit |  |
| `TestPeriodicSnapshotSchema` | Unit |  |

## `internal/resources/pool`  (9)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccPoolDataSource_basic` | Acceptance · Tier 1 | TestAccPoolDataSource_basic reads the acceptance-test pool (acctest.TestPool(), default "tank") via the truenas_pool data source and checks that the name round-trips and a status is reported. |
| `TestAccPool_basic` | Acceptance · conditional skip | TestAccPool_basic would exercise full create/update/import/destroy of a truenas_pool resource, but pool.create requires dedicated spare disks that are not safe to assume are present (or blank) on any given target box. |
| `TestAutotrimParsedUnmarshalJSON` | Unit | TestAutotrimParsedUnmarshalJSON is a regression test for a live acceptance failure: pool.query / pool.get_instance return autotrim as a ZFS property object whose "parsed" field is the string "on"/"off", not a JSON bool. |
| `TestPoolAPIAutotrimPropertyObjectDecode` | Unit | TestPoolAPIAutotrimPropertyObjectDecode verifies that a full poolAPI response, shaped like the live pool.query payload (autotrim.parsed as a string), decodes without error and maps to the correct bool. |
| `TestPoolAPIPayload` | Unit | TestPoolAPIPayload constructs a PoolModel with a MIRROR data vdev and verifies that apiPayload produces the correct wire-format structure. |
| `TestPoolDataSourceModelTopologyIsNullable` | Unit | TestPoolDataSourceModelTopologyIsNullable is a regression test for a live acceptance failure: "Received null value, however the target type cannot handle null values. |
| `TestPoolResponseToDataSourceModel` | Unit | TestPoolResponseToDataSourceModel verifies that responseToDataSourceModel maps a TrueNAS API response (with topology children) into a non-null types.Object matching topologyAttrTypes, using the same conversion logic as the resource's responseToModel. |
| `TestPoolResponseToModel` | Unit | TestPoolResponseToModel verifies that responseToModel correctly maps a TrueNAS API response (with topology children) into Terraform state. |
| `TestPoolSchema` | Unit | TestPoolSchema verifies that resourceSchema returns a schema containing all expected top-level attributes. |

## `internal/resources/privilege`  (18)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccPrivilege_basic` | Acceptance · Tier 1 | TestAccPrivilege_basic creates a truenas_group fixture and a privilege referencing its gid via local_groups, checks attributes, adds a role in place, imports the privilege by its numeric id, and verifies destruction of both the privilege and the group fixture. |
| `TestApiPayload_AllFieldsSet` | Unit | TestApiPayload_AllFieldsSet verifies the payload built when every field is known: local_groups/ds_groups/roles come through as flat GID/role lists. |
| `TestApiPayload_UnsetListsBecomeEmpty` | Unit | TestApiPayload_UnsetListsBecomeEmpty verifies that null local_groups, ds_groups, and roles are sent as empty lists (not omitted), matching privilege.create/update's own default of [] and the group resource's basePayload convention. |
| `TestGroupRefsToGIDs_EmbeddedGroupEntry` | Unit | TestGroupRefsToGIDs_EmbeddedGroupEntry verifies the shape actually observed on privilege.create/query/update/get_instance reads: an array of embedded GroupEntry objects (full group details, including "gid"). |
| `TestGroupRefsToGIDs_Empty` | Unit | TestGroupRefsToGIDs_Empty verifies an empty array decodes to an empty (non-nil) slice, matching privilege.query's own [] default for a privilege with no groups assigned. |
| `TestGroupRefsToGIDs_Invalid` | Unit | TestGroupRefsToGIDs_Invalid verifies malformed JSON returns an error rather than silently producing an empty/partial list. |
| `TestGroupRefsToGIDs_MultipleEntries` | Unit | TestGroupRefsToGIDs_MultipleEntries verifies multiple embedded group objects all decode in order. |
| `TestGroupRefsToGIDs_NullOrMissing` | Unit | TestGroupRefsToGIDs_NullOrMissing verifies a JSON null (or zero-length RawMessage, e.g. |
| `TestGroupRefsToGIDs_UnmappedEntrySkipped` | Unit | TestGroupRefsToGIDs_UnmappedEntrySkipped verifies the UnmappedGroupEntry shape (a group reference that couldn't be resolved — gid null, sid set, group null, per the middleware's own schema for ds_groups/local_groups) is skipped rather than producing a zero-value GID or an error: it cannot be represented as a GID, and silently emitting 0 would be worse than omitting it (0 could collide with a real GID in principle, and more importantly would misrepresent an unmapped/unknown entry as a configured one). |
| `TestPrivilegeDataSourceModel_MatchesSchema` | Unit | TestPrivilegeDataSourceModel_MatchesSchema verifies that every tfsdk tag on PrivilegeDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestPrivilegeSchema_BuiltinNameIsComputedOnly` | Unit | TestPrivilegeSchema_BuiltinNameIsComputedOnly verifies "builtin_name" is Computed-only: the API generates it, and this resource never writes it (it only ever creates custom, non-builtin privileges). |
| `TestPrivilegeSchema_GroupListsAreOptionalComputedInt64` | Unit | TestPrivilegeSchema_GroupListsAreOptionalComputedInt64 verifies local_groups and ds_groups are Optional+Computed lists of Int64 (GIDs), matching the brief's "list int GID" spec and privilege.create's own default of []. |
| `TestPrivilegeSchema_IDIsComputed` | Unit | TestPrivilegeSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestPrivilegeSchema_RequiredFields` | Unit | TestPrivilegeSchema_RequiredFields verifies "name" and "web_shell" are Required, matching privilege.create's own "required" list. |
| `TestPrivilegeSchema_RolesIsOptionalComputedStringList` | Unit | TestPrivilegeSchema_RolesIsOptionalComputedStringList verifies "roles" is an Optional+Computed list of strings. |
| `TestResponseToDataSourceModel_QueryShape` | Unit | TestResponseToDataSourceModel_QueryShape mirrors TestResponseToModel_QueryShape for the datasource model. |
| `TestResponseToModel_BuiltinPrivilege` | Unit | TestResponseToModel_BuiltinPrivilege verifies a builtin privilege (as returned by an unfiltered privilege.query, which always includes the three server-shipped privileges) maps builtin_name through correctly. |
| `TestResponseToModel_QueryShape` | Unit | TestResponseToModel_QueryShape verifies responseToModel against the exact shape observed from a live privilege.create/query call: local_groups holding one embedded group object, ds_groups empty, roles populated. |

## `internal/resources/replication`  (24)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccReplication_RemoteSSH` | Acceptance · Tier 1 | TestAccReplication_RemoteSSH exercises transport = "SSH" end to end: an ed25519 truenas_keychain_ssh_keypair + truenas_keychain_ssh_connection fixture pair (HCL-referenced, per the loopback pattern established by keychain_ssh_connection's own acceptance test), a throwaway truenas_user fixture (also HCL-referenced) whose sshpubkey and sudo_commands_nopasswd are set directly in config rather than mutating any pre-existing box account, and a PUSH replication task over that connection — full contract (create with compression/speed_limit/sudo set, in-place update, import), plus a genuine replication.run round trip against a real snapshot (see runReplicationAndVerify's doc comment for why a full run was chosen over a state-only check). |
| `TestAccReplication_basic` | Acceptance · Tier 1 | TestAccReplication_basic tests create, update, and import of a LOCAL push replication task. |
| `TestNameRegexConflict` | Unit | TestNameRegexConflict verifies the mutual-exclusion boolean logic used by ReplicationResource.ValidateConfig: name_regex conflicts with a non-empty naming_schema and/or also_include_naming_schema, but not when only one of the two attribute groups is set, and unknown values (not yet known at plan time) are treated as absent so partially-unknown configs don't falsely trip the check. |
| `TestPeriodicSnapshotTaskIDs` | Unit |  |
| `TestPeriodicSnapshotTaskIDs_Empty` | Unit |  |
| `TestReplicationApiPayload_OmitsUnsetOptionals` | Unit |  |
| `TestReplicationPayload_CompressionSpeedLimitNullBecomesNil` | Unit | TestReplicationPayload_CompressionSpeedLimitNullBecomesNil verifies the unset (null) case always sends an explicit nil rather than omitting the key — matching the lifetime_value/lifetime_unit nil-clearing pattern, so an in-place update can clear a previously-set compression/speed_limit. |
| `TestReplicationPayload_CompressionSpeedLimitSet` | Unit | TestReplicationPayload_CompressionSpeedLimitSet verifies compression and speed_limit pass through to the payload when set (SSH-only fields). |
| `TestReplicationPayload_LifetimeNonZeroValuesPassThrough` | Unit |  |
| `TestReplicationPayload_LifetimeZeroValuesBecomeNull` | Unit |  |
| `TestReplicationPayload_LocalTransport` | Unit |  |
| `TestReplicationPayload_NameRegexOmitsNamingSchema` | Unit |  |
| `TestReplicationPayload_NoNameRegexIncludesNamingSchema` | Unit |  |
| `TestReplicationPayload_PeriodicSnapshotTasksNilBecomesEmptySlice` | Unit |  |
| `TestReplicationPayload_SSHCredentialsNonZero` | Unit |  |
| `TestReplicationPayload_ScheduleNull` | Unit |  |
| `TestReplicationPayload_ScheduleSet` | Unit |  |
| `TestReplicationSchema` | Unit |  |
| `TestResponseToModel_CompressionSpeedLimit` | Unit | TestResponseToModel_CompressionSpeedLimit verifies both the nil (LOCAL task, or SSH task with neither set) and populated (SSH task) cases decode to null / concrete values respectively. |
| `TestResponseToModel_ScheduleNilAndLifetimeNil` | Unit |  |
| `TestResponseToModel_ScheduleSetAndEmbeddedTasks` | Unit |  |
| `TestSSHCredentialsID` | Unit |  |
| `TestSSHOnlyFieldsWithoutSSH` | Unit | TestSSHOnlyFieldsWithoutSSH exercises the preflight enforced in ValidateConfig: compression/speed_limit are only valid for transport = "SSH". |
| `TestTransportSSHCredentialsMismatch` | Unit | TestTransportSSHCredentialsMismatch exercises the preflight enforced in ValidateConfig: transport = "SSH" requires ssh_credentials, transport = "LOCAL" (explicit or defaulted from an unset/unknown config value) forbids it. |

## `internal/resources/replication_config`  (9)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccReplicationConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccReplicationConfigDataSource_basic reads the current TrueNAS replication configuration through the truenas_replication_config datasource only. |
| `TestAccReplicationConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccReplicationConfig_setAndRestore drives the singleton truenas_replication_config resource's "max_parallel_replication_tasks" field through a test value, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: ReplicationConfigResource.Delete calls only this pure function. |
| `TestReplicationConfigDataSourceModel_MatchesSchema` | Unit | TestReplicationConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on ReplicationConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestReplicationConfigSchema_IDIsComputed` | Unit | TestReplicationConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestReplicationConfigSchema_MaxParallelOptionalComputed` | Unit | TestReplicationConfigSchema_MaxParallelOptionalComputed verifies that max_parallel_replication_tasks is Optional+Computed with a UseStateForUnknown plan modifier. |
| `TestResponseToDataSourceModel_NilMapsToZeroSentinel` | Unit | TestResponseToDataSourceModel_NilMapsToZeroSentinel mirrors TestResponseToModel_NilMapsToZeroSentinel for the datasource model. |
| `TestResponseToModel_NilMapsToZeroSentinel` | Unit | TestResponseToModel_NilMapsToZeroSentinel verifies that a nil max_parallel_replication_tasks on the wire maps to 0 (the "unlimited" sentinel), and that a non-nil value maps through unchanged. |
| `TestUpdatePayload_ThreeWay` | Unit | TestUpdatePayload_ThreeWay verifies the three-way behavior for max_parallel_replication_tasks: null/unknown omits the key entirely, an explicit 0 sends JSON nil (meaning unlimited), and any other value N sends N. |

## `internal/resources/reporting_exporter`  (9)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccReportingExporter_basic` | Acceptance · Tier 1 | TestAccReportingExporter_basic exercises the full Tier 1 contract for a GRAPHITE reporting exporter: create pointed at the 192.0.2.0/24 TEST-NET-1 documentation range (RFC 5737) so nothing real is ever contacted, update in place, import, and destroy verification via a live reporting.exporters.query. |
| `TestApiPayload_FullySet` | Unit | TestApiPayload_FullySet verifies apiPayload includes name, enabled, and a full attributes sub-map (with the fixed exporter_type constant injected), matching the probed reporting.exporters.create shape. |
| `TestAttributesPayload_UnsetOptionalsOmitted` | Unit | TestAttributesPayload_UnsetOptionalsOmitted verifies that the Optional+Computed GRAPHITE fields are omitted from the attributes payload when null/unknown, leaving only exporter_type + the three Required fields — so the TrueNAS-side default takes effect for the rest. |
| `TestReportingExporterDataSourceModel_MatchesSchema` | Unit | TestReportingExporterDataSourceModel_MatchesSchema verifies that every tfsdk tag on ReportingExporterDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestResponseToDataSourceModel_ProbedShape` | Unit | TestResponseToDataSourceModel_ProbedShape mirrors TestResponseToModel_ProbedShape for the datasource model. |
| `TestResponseToModel_ProbedShape` | Unit | TestResponseToModel_ProbedShape verifies responseToModel against the exact shape observed from a live reporting.exporters.create/get_instance call, including the server-filled GRAPHITE defaults. |
| `TestSchema_AttributesShape` | Unit | TestSchema_AttributesShape verifies the nested "attributes" block is Required at the top level, with destination_ip/destination_port/namespace Required and the remaining GRAPHITE fields Optional+Computed, matching the probed reporting.exporters.exporter_schemas GRAPHITE variant. |
| `TestSchema_IDComputedUseStateForUnknown` | Unit | TestSchema_IDComputedUseStateForUnknown verifies that the "id" attribute is Computed-only. |
| `TestSchema_TopLevelRequiredFields` | Unit | TestSchema_TopLevelRequiredFields verifies "name" and "enabled" are Required, matching reporting.exporters.create's own "required" list (no server-side default for either). |

## `internal/resources/resilver_config`  (10)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccResilverConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccResilverConfigDataSource_basic reads the current TrueNAS resilver configuration through the truenas_resilver_config datasource only. |
| `TestAccResilverConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccResilverConfig_setAndRestore drives the singleton truenas_resilver_config resource's "enabled" field through a toggled value and back to the value read from the box before the test ran, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable. |
| `TestResilverConfigDataSourceModel_MatchesSchema` | Unit | TestResilverConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on ResilverConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestResilverConfigSchema_IDIsComputed` | Unit | TestResilverConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestResilverConfigSchema_OtherFieldsAreOptionalComputed` | Unit | TestResilverConfigSchema_OtherFieldsAreOptionalComputed verifies that begin, end, enabled, and weekday are all Optional+Computed with UseStateForUnknown plan modifiers. |
| `TestResponseToModel` | Unit | TestResponseToModel verifies responseToModel against the shape probed from a live pool.resilver.config call. |
| `TestResponseToModel_NilWeekdayBecomesEmptyList` | Unit | TestResponseToModel_NilWeekdayBecomesEmptyList verifies that a nil weekday slice from the API maps to an empty (non-null) list, matching the nil-guard convention used elsewhere for API-returned lists. |
| `TestUpdatePayload_AllFieldsSet` | Unit | TestUpdatePayload_AllFieldsSet verifies that updatePayload includes every field with the exact keys observed in the pool.resilver.update probe. |
| `TestUpdatePayload_UnsetOptionalsOmitted` | Unit | TestUpdatePayload_UnsetOptionalsOmitted verifies that every field is omitted when null/unknown, so the current TrueNAS-side value is left unchanged rather than overwritten with a zero value. |

## `internal/resources/rsync_task`  (20)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccRsyncTask_basic` | Acceptance · Tier 1 | TestAccRsyncTask_basic creates a dataset fixture and a MODULE-mode rsync task pointed at an unreachable remote host, checks its attributes, updates desc in place, imports the task by its numeric id, and verifies destruction of both the task and the dataset fixture. |
| `TestApiPayload_MODULEMode` | Unit | TestApiPayload_MODULEMode verifies the payload built for a MODULE-mode task: remotehost/remotemodule are included, ssh_credentials/remoteport are omitted (both null), matching the probed create shape. |
| `TestApiPayload_SSHMode` | Unit | TestApiPayload_SSHMode verifies the payload built for an SSH-mode task: remotehost/remoteport/ssh_credentials are all included, remotemodule is omitted (irrelevant to SSH mode and left null). |
| `TestApiPayload_UnsetOptionalsOmitted` | Unit | TestApiPayload_UnsetOptionalsOmitted verifies that every Optional field is omitted from the payload when null/unknown, leaving only the two Required fields "path" and "user" — so TrueNAS-side defaults take effect. |
| `TestApiPayload_ValidateRPathAndSSHKeyscan` | Unit | TestApiPayload_ValidateRPathAndSSHKeyscan verifies both write-only flags are included when known, alongside the rest of the payload. |
| `TestDecodeSSHCredentialsID_BareInteger` | Unit | TestDecodeSSHCredentialsID_BareInteger verifies defensive support for a bare integer, in case a future API version returns one directly. |
| `TestDecodeSSHCredentialsID_EmbeddedObject` | Unit | TestDecodeSSHCredentialsID_EmbeddedObject verifies the shape actually observed on rsynctask.query/get_instance reads: an embedded KeychainCredentialEntry object. |
| `TestDecodeSSHCredentialsID_Invalid` | Unit | TestDecodeSSHCredentialsID_Invalid verifies an undecodable shape returns an error rather than silently defaulting to nil. |
| `TestDecodeSSHCredentialsID_Null` | Unit | TestDecodeSSHCredentialsID_Null verifies a JSON null decodes to (nil, nil). |
| `TestResponseToDataSourceModel_MODULEQueryShape` | Unit | TestResponseToDataSourceModel_MODULEQueryShape mirrors TestResponseToModel_MODULEQueryShape for the datasource model. |
| `TestResponseToModel_MODULEQueryShape` | Unit | TestResponseToModel_MODULEQueryShape verifies responseToModel against the exact shape observed from a live rsynctask.create/query call in MODULE mode: remoteport and ssh_credentials both null, remotehost/remotemodule plain strings. |
| `TestResponseToModel_SSHCredentialsEmbeddedObject` | Unit | TestResponseToModel_SSHCredentialsEmbeddedObject verifies responseToModel correctly unwraps the embedded KeychainCredentialEntry shape into a plain int64 ID. |
| `TestRsyncTaskDataSourceModel_MatchesSchema` | Unit | TestRsyncTaskDataSourceModel_MatchesSchema verifies that every tfsdk tag on RsyncTaskDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestRsyncTaskSchema_ExtraIsList` | Unit | TestRsyncTaskSchema_ExtraIsList verifies "extra" is an Optional+Computed list of strings. |
| `TestRsyncTaskSchema_IDIsComputed` | Unit | TestRsyncTaskSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestRsyncTaskSchema_NullableIntsAreOptionalComputed` | Unit | TestRsyncTaskSchema_NullableIntsAreOptionalComputed verifies remoteport and ssh_credentials — the wire-nullable int fields — are Optional+Computed so a null API value doesn't fight the plan. |
| `TestRsyncTaskSchema_OptionalComputedBoolFlags` | Unit | TestRsyncTaskSchema_OptionalComputedBoolFlags verifies the rsync bool flags carrying TrueNAS-side defaults are all Optional+Computed. |
| `TestRsyncTaskSchema_RequiredFields` | Unit | TestRsyncTaskSchema_RequiredFields verifies "path" and "user" are Required, matching rsynctask.create's own "required" list. |
| `TestRsyncTaskSchema_ScheduleShape` | Unit | TestRsyncTaskSchema_ScheduleShape verifies the nested "schedule" attribute is Optional+Computed at the top level with Required string sub-fields, matching scrub_task's convention. |
| `TestRsyncTaskSchema_WriteOnlyFlagsAreOptionalOnly` | Unit | TestRsyncTaskSchema_WriteOnlyFlagsAreOptionalOnly verifies validate_rpath and ssh_keyscan are plain Optional (no Computed): the API never returns them, so nothing can compute a value for an unset config. |

## `internal/resources/scrub_task`  (10)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccScrubTask_basic` | Acceptance · conditional skip | TestAccScrubTask_basic creates a scrub schedule on the test pool, checks its attributes, updates threshold in place, imports it by id, and verifies destruction via a live pool.scrub.query. |
| `TestApiPayload_AllFieldsSet` | Unit | TestApiPayload_AllFieldsSet verifies that apiPayload includes every field — pool, threshold, description, schedule, enabled — when all are known, with the exact keys observed in the pool.scrub.create probe. |
| `TestApiPayload_UnsetOptionalsOmitted` | Unit | TestApiPayload_UnsetOptionalsOmitted verifies that threshold, description, schedule, and enabled are omitted from the payload when null or unknown, leaving only the Required "pool" field — so the TrueNAS-side defaults (threshold=35, description="", enabled=true, schedule=00 00 * * 7) take effect instead of an explicit zero value being sent. |
| `TestResponseToDataSourceModel_QueryShape` | Unit | TestResponseToDataSourceModel_QueryShape mirrors TestResponseToModel_QueryShape for the datasource model. |
| `TestResponseToModel_QueryShape` | Unit | TestResponseToModel_QueryShape verifies responseToModel against the exact shape observed from a live pool.scrub.query call: "pool" is a plain integer (never an embedded object), accompanied by a "pool_name" string. |
| `TestScrubTaskDataSourceModel_MatchesSchema` | Unit | TestScrubTaskDataSourceModel_MatchesSchema verifies that every tfsdk tag on ScrubTaskDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestScrubTaskSchema_IDIsComputed` | Unit | TestScrubTaskSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestScrubTaskSchema_OptionalComputedFields` | Unit | TestScrubTaskSchema_OptionalComputedFields verifies threshold, description, schedule, and enabled are Optional+Computed, matching the TrueNAS-side defaults observed in the pool.scrub.create probe. |
| `TestScrubTaskSchema_PoolNameIsComputedOnly` | Unit | TestScrubTaskSchema_PoolNameIsComputedOnly verifies "pool_name" is a Computed-only extra field surfaced from the pool.scrub.query/get_instance response. |
| `TestScrubTaskSchema_PoolRequiresReplace` | Unit | TestScrubTaskSchema_PoolRequiresReplace verifies that "pool" is Required and carries RequiresReplace, since a scrub task cannot be moved to a different pool in place (pool.scrub.update's "pool" is documented as changeable, but the provider treats it as immutable to keep the resource tied to a single pool for its whole lifecycle). |

## `internal/resources/service`  (8)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccService_ftp` | Acceptance · Tier 1 | TestAccService_ftp enables and then restores the "ftp" service's enabled (autostart) flag. |
| `TestResponseToModel_AllFields` | Unit | TestResponseToModel_AllFields verifies that all fields are mapped correctly. |
| `TestResponseToModel_IDIsServiceName` | Unit | TestResponseToModel_IDIsServiceName verifies that the Terraform ID is set to the service name string (not a numeric ID). |
| `TestResponseToModel_Running` | Unit | TestResponseToModel_Running verifies that State=="RUNNING" maps to Running=true and any other state maps to Running=false. |
| `TestServiceSchema_EnabledIsOptionalComputed` | Unit | TestServiceSchema_EnabledIsOptionalComputed verifies that "enabled" is both Optional and Computed. |
| `TestServiceSchema_IDIsString` | Unit | TestServiceSchema_IDIsString verifies that the "id" attribute is a StringAttribute (not Int64), because services use the service name as their Terraform ID. |
| `TestServiceSchema_NameIsRequiresReplace` | Unit | TestServiceSchema_NameIsRequiresReplace verifies that "name" carries the RequiresReplace plan modifier, forcing resource recreation if the service name changes. |
| `TestServiceSchema_RunningIsOptionalComputed` | Unit | TestServiceSchema_RunningIsOptionalComputed verifies that "running" is both Optional (users may set it) and Computed (TrueNAS provides it when unset). |

## `internal/resources/smb`  (10)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccSMBShare_basic` | Acceptance · Tier 1 | TestAccSMBShare_basic creates a dataset fixture and an SMB share on its mountpoint, checks its attributes, updates comment and abe in place, imports it by its numeric id, and verifies destruction of both the share and the dataset fixture. |
| `TestResponseToModel_LegacyShare` | Unit | TestResponseToModel_LegacyShare verifies that responseToModel decodes the nested TrueNAS 26.0 response shape (top-level readonly/ access_based_share_enumeration, options.* for legacy flags) into the flat SMBModel fields. |
| `TestResponseToModel_LegacyShare_RawJSONFixture` | Unit | TestResponseToModel_LegacyShare_RawJSONFixture decodes a raw JSON payload shaped exactly like the sharing.smb.create / get_instance response on TrueNAS 26.0 (per the middleware schema dump: top-level readonly/ access_based_share_enumeration/purpose, and legacy flags nested under options for the LEGACY_SHARE variant). |
| `TestResponseToModel_NonLegacyShare` | Unit | TestResponseToModel_NonLegacyShare verifies that for a non-LEGACY_SHARE purpose, responseToModel zeroes out the legacy fields (since they are not present/meaningful in the options variant for e.g. |
| `TestSMBApiPayload_LegacyFlagsGuardedByNullUnknown` | Unit | TestSMBApiPayload_LegacyFlagsGuardedByNullUnknown verifies that legacy flags are only added to options when they are neither null nor unknown in the model (guards against clobbering unrelated server-side defaults), and that null hostsallow/hostsdeny are simply omitted (not sent as nil). |
| `TestSMBApiPayload_NonLegacyPurposeOmitsLegacyFlags` | Unit | TestSMBApiPayload_NonLegacyPurposeOmitsLegacyFlags verifies that when the caller sets purpose to a known non-LEGACY_SHARE value, options only contains purpose (variant defaults apply server-side) and none of the legacy flags are sent, either at the top level or in options. |
| `TestSMBApiPayload_PurposeDefaultsToLegacy` | Unit | TestSMBApiPayload_PurposeDefaultsToLegacy verifies that apiPayload defaults purpose to LEGACY_SHARE (both top-level and options.purpose) when the caller hasn't set a purpose, and that the legacy flags are nested under options with the discriminator. |
| `TestSMBApiPayload_PurposeDefaultsToLegacyWhenUnrecognized` | Unit | TestSMBApiPayload_PurposeDefaultsToLegacyWhenUnrecognized verifies that an unrecognized (e.g. |
| `TestSMBApiPayload_TopLevel` | Unit | TestSMBApiPayload_TopLevel verifies apiPayload produces the TrueNAS 26.0 top-level keys: path/name/comment/enabled/browsable/readonly/ access_based_share_enumeration/purpose/options, and that the legacy 24.x top-level keys (ro, abe, hostsallow, etc.) are gone from the top level. |
| `TestSMBSchema` | Unit | TestSMBSchema verifies that the resource schema has the expected attributes and that key attributes have the correct types. |

## `internal/resources/smb_config`  (20)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccSMBConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccSMBConfigDataSource_basic reads the current TrueNAS SMB configuration through the truenas_smb_config datasource only. |
| `TestAccSMBConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccSMBConfig_setAndRestore drives the singleton truenas_smb_config resource's "description" field (an inert, cosmetic label) through a test value and back to the value read from the box before the test ran, then imports it. |
| `TestApplyPost2600FieldsSupport_ProbeErrorStripsFields` | Unit | TestApplyPost2600FieldsSupport_ProbeErrorStripsFields verifies the fail-closed direction of applyPost2600FieldsSupport: when the version probe itself errors (ServerVersion returns err != nil), the gate must strip stateful_failover/minimum_protocol/search_protocols from the payload rather than keep them, since keeping them on an unprobed (majority pre-26.0) target would send fields smb.update rejects with its generic "Extra inputs are not permitted" error. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: SMBConfigResource.Delete calls only this pure function. |
| `TestPost2600FieldsSupported` | Unit | TestPost2600FieldsSupported verifies the pure version-comparison gate for smb.update's stateful_failover/minimum_protocol/search_protocols fields, including the exact boundary and malformed-input edge cases. |
| `TestResponseToDataSourceModel_AdminGroupNilBecomesEmptyString` | Unit | TestResponseToDataSourceModel_AdminGroupNilBecomesEmptyString mirrors the admin_group nil handling for the datasource model. |
| `TestResponseToDataSourceModel_ListsNilBecomeEmptyList` | Unit | TestResponseToDataSourceModel_ListsNilBecomeEmptyList mirrors the same nil-to-empty-list handling for the datasource model. |
| `TestResponseToModel_AdminGroupNilBecomesEmptyString` | Unit | TestResponseToModel_AdminGroupNilBecomesEmptyString verifies that API nil (JSON null) for admin_group maps to an empty string in state. |
| `TestResponseToModel_AdminGroupSet` | Unit | TestResponseToModel_AdminGroupSet verifies that a non-nil API value for admin_group is carried through as-is. |
| `TestResponseToModel_ListsNilBecomeEmptyList` | Unit | TestResponseToModel_ListsNilBecomeEmptyList verifies that nil list fields from the API map to empty (non-null) lists in the model, for all three list fields. |
| `TestResponseToModel_ListsSet` | Unit | TestResponseToModel_ListsSet verifies that non-nil list fields from the API are carried through as-is. |
| `TestSMBConfigDataSourceModel_MatchesSchema` | Unit | TestSMBConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on SMBConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestSMBConfigSchema_AllOtherFieldsOptionalComputed` | Unit | TestSMBConfigSchema_AllOtherFieldsOptionalComputed verifies that every writable field is Optional+Computed with a plan modifier, matching the "all writable fields Optional+Computed + UseStateForUnknown" contract in the task brief. |
| `TestSMBConfigSchema_IDIsComputed` | Unit | TestSMBConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestSMBConfigSchema_ServerSIDIsComputedOnly` | Unit | TestSMBConfigSchema_ServerSIDIsComputedOnly verifies that server_sid is Computed-only (never Optional): it is a stable server-assigned value that must never be part of the update payload. |
| `TestUpdatePayload_AdminGroupThreeWay` | Unit | TestUpdatePayload_AdminGroupThreeWay verifies the three-way nullable handling for admin_group: omitted when null/unknown, sent as nil when explicitly set to "", and sent as the value otherwise. |
| `TestUpdatePayload_AllKnownFieldsSent` | Unit | TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes every guarded field when its model value is known, and that server_sid and the three post-26.0 fields (handled separately by applyPost2600FieldsSupport) are excluded even though the model carries known values for all of them. |
| `TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice` | Unit | TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice verifies that a known but empty list is sent as an empty slice, not nil/null, for the two list fields updatePayload still handles directly (search_protocols moved to applyPost2600FieldsSupport). |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any field whose model value is null or unknown. |
| `TestUpdatePayload_ServerSIDNeverInPayload` | Unit | TestUpdatePayload_ServerSIDNeverInPayload verifies that server_sid is never included in the update payload, even though it holds a known value: it is Computed-only and TrueNAS does not accept it as an smb.update argument. |

## `internal/resources/snapshot`  (3)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccSnapshot_basic` | Acceptance · Tier 1 | TestAccSnapshot_basic creates a dataset fixture, snapshots it, imports the snapshot by "dataset@name", and verifies destruction. |
| `TestSnapshotIDFormat` | Unit | TestSnapshotIDFormat verifies that responseToModel produces the "dataset@snapname" ID format expected from the TrueNAS API. |
| `TestSnapshotSchema` | Unit | TestSnapshotSchema verifies that id is Computed and that dataset/name are Required with RequiresReplace plan modifiers (i.e., ForceNew semantics). |

## `internal/resources/snmp_config`  (17)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccSNMPConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccSNMPConfigDataSource_basic reads the current TrueNAS SNMP configuration through the truenas_snmp_config datasource only. |
| `TestAccSNMPConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccSNMPConfig_setAndRestore drives the singleton truenas_snmp_config resource's "location" field (a cosmetic, low-risk descriptive string) through a test value and back to the value read from the box before the test ran, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: SNMPConfigResource.Delete calls only this pure function. |
| `TestResponseToDataSourceModel_V3PrivProtoNilBecomesEmptyString` | Unit | TestResponseToDataSourceModel_V3PrivProtoNilBecomesEmptyString mirrors the same nil-to-empty-string handling for the datasource model. |
| `TestResponseToModel_SecretsNeverSet` | Unit | TestResponseToModel_SecretsNeverSet verifies that responseToModel never writes to m.V3Password or m.V3PrivPassphrase, regardless of their prior value: both are write-only and the API never returns usable values for them. |
| `TestResponseToModel_V3PrivProtoNilBecomesEmptyString` | Unit | TestResponseToModel_V3PrivProtoNilBecomesEmptyString verifies that a nil v3_privproto from the API maps to an empty string in the model. |
| `TestResponseToModel_V3PrivProtoSet` | Unit | TestResponseToModel_V3PrivProtoSet verifies that a non-nil v3_privproto from the API is carried through as-is. |
| `TestSNMPConfigDataSourceModel_MatchesSchema` | Unit | TestSNMPConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on SNMPConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestSNMPConfigDataSourceModel_NoSecretFields` | Unit | TestSNMPConfigDataSourceModel_NoSecretFields verifies that SNMPConfigDataSourceModel has no v3_password or v3_privpassphrase field: the API never returns usable values for either, so datasource attributes for them would always read as empty/unknown. |
| `TestSNMPConfigSchema_CommunityIsSensitive` | Unit | TestSNMPConfigSchema_CommunityIsSensitive verifies that "community" is Optional+Computed+Sensitive: it is API-echoed (snmp.config returns it), so unlike v3_password/v3_privpassphrase it must stay Computed and persisted in state (not WriteOnly), but it is still a secret and must not print in plan output. |
| `TestSNMPConfigSchema_IDIsComputed` | Unit | TestSNMPConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestSNMPConfigSchema_OtherFieldsAreOptionalComputed` | Unit | TestSNMPConfigSchema_OtherFieldsAreOptionalComputed verifies that every non-secret, non-id field is Optional+Computed with UseStateForUnknown plan modifiers, matching the "non-secret fields" contract in the task brief. |
| `TestSNMPConfigSchema_SecretsAreSensitiveWriteOnly` | Unit | TestSNMPConfigSchema_SecretsAreSensitiveWriteOnly verifies that "v3_password" and "v3_privpassphrase" are Optional + Sensitive, and specifically NOT Computed (write-only: never read back from TrueNAS, so they must not participate in drift detection). |
| `TestUpdatePayload_AllKnownFieldsSent` | Unit | TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes every guarded non-secret, non-nullable field when its model value is known. |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any field whose model value is null or unknown, for every guarded field except v3_privproto/v3_password/v3_privpassphrase (which have their own tests below). |
| `TestUpdatePayload_SecretsOnlyWhenSet` | Unit | TestUpdatePayload_SecretsOnlyWhenSet verifies that v3_password and v3_privpassphrase are included only when they have a known, non-null value, and are otherwise omitted entirely (never sent as an empty string or null). |
| `TestUpdatePayload_V3PrivProtoThreeWay` | Unit | TestUpdatePayload_V3PrivProtoThreeWay verifies the three-way nullable handling for v3_privproto: omitted when null/unknown, sent as nil when explicitly set to "", and sent as the value otherwise. |

## `internal/resources/ssh_config`  (12)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccSSHConfigDataSource_basic` | Acceptance · conditional skip | TestAccSSHConfigDataSource_basic reads the current TrueNAS SSH configuration through the truenas_ssh_config datasource only. |
| `TestAccSSHConfig_basic` | Acceptance · conditional skip | TestAccSSHConfig_basic is intentionally skipped by default. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: SSHConfigResource.Delete calls only this pure function. |
| `TestResponseToDataSourceModel_ListsNilBecomeEmptyList` | Unit | TestResponseToDataSourceModel_ListsNilBecomeEmptyList mirrors the same nil-to-empty-list handling for the datasource model. |
| `TestResponseToModel_ListsNilBecomeEmptyList` | Unit | TestResponseToModel_ListsNilBecomeEmptyList verifies that nil list fields from the API map to empty (non-null) lists in the model, for all three list fields. |
| `TestResponseToModel_ListsSet` | Unit | TestResponseToModel_ListsSet verifies that non-nil list fields from the API are carried through as-is. |
| `TestSSHConfigDataSourceModel_MatchesSchema` | Unit | TestSSHConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on SSHConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestSSHConfigSchema_AllFieldsOptionalComputed` | Unit | TestSSHConfigSchema_AllFieldsOptionalComputed verifies that every non-id field is Optional+Computed with a plan modifier, matching the "all fields Optional+Computed + UseStateForUnknown" contract in the task brief. |
| `TestSSHConfigSchema_IDIsComputed` | Unit | TestSSHConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestUpdatePayload_AllKnownFieldsSent` | Unit | TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes every guarded field when its model value is known. |
| `TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice` | Unit | TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice verifies that a known but empty list is sent as an empty slice, not nil/null, for all three list fields. |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any field whose model value is null or unknown. |

## `internal/resources/static_route`  (4)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccStaticRoute_basic` | Acceptance · Tier 1 | TestAccStaticRoute_basic tests create, update, and import of a static route. |
| `TestStaticRouteApiPayload` | Unit | TestStaticRouteApiPayload verifies that apiPayload produces a map with exactly the expected keys and values. |
| `TestStaticRouteResponseToModel` | Unit | TestStaticRouteResponseToModel verifies that responseToModel populates all fields from the staticRouteAPI struct correctly. |
| `TestStaticRouteSchema` | Unit | TestStaticRouteSchema verifies that the resource schema has the expected attributes with correct types. |

## `internal/resources/system_advanced`  (23)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccSystemAdvancedDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccSystemAdvancedDataSource_basic reads the current TrueNAS system advanced configuration through the truenas_system_advanced datasource only. |
| `TestAccSystemAdvanced_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccSystemAdvanced_setAndRestore drives the singleton truenas_system_advanced resource's "motd" field (a cosmetic message-of-the-day string) through a test value and back to the value read from the box before the test ran, then imports it. |
| `TestApplyNvidiaSupport_ProbeErrorStripsField` | Unit | TestApplyNvidiaSupport_ProbeErrorStripsField verifies the fail-closed direction of applyNvidiaSupport: when the version probe itself errors (ServerVersion returns err != nil), the gate must strip "nvidia" from the payload rather than keep it, since keeping it on an unprobed (majority pre-26.0) target would send a field system.advanced.update rejects with its generic "Extra inputs are not permitted" error. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: SystemAdvancedResource.Delete calls only this pure function. |
| `TestNvidiaSupported` | Unit | TestNvidiaSupported verifies the pure version-comparison gate for system.advanced.update's "nvidia" field, including the exact boundary and malformed-input edge cases. |
| `TestResponseToDataSourceModel_NoSedPasswdField` | Unit | TestResponseToDataSourceModel_NoSedPasswdField verifies (via reflection) that SystemAdvancedDataSourceModel has no sed_passwd field at all. |
| `TestResponseToModel_ListsNilMapToEmpty` | Unit | TestResponseToModel_ListsNilMapToEmpty verifies that nil list fields from the API (syslogservers, isolated_gpu_pci_ids) map to empty (non-null) Terraform lists. |
| `TestResponseToModel_MapsPlainFields` | Unit | TestResponseToModel_MapsPlainFields verifies that non-nullable, non-pointer fields are copied through unchanged, including the two Computed-only fields anonstats_token and isolated_gpu_pci_ids. |
| `TestResponseToModel_OverprovisionMapsToNull` | Unit | TestResponseToModel_OverprovisionMapsToNull verifies that a nil overprovision on the wire maps to Int64Null (not a zero value), and that a non-nil value maps through unchanged. |
| `TestResponseToModel_SedPasswdNeverSet` | Unit | TestResponseToModel_SedPasswdNeverSet verifies that responseToModel never touches SedPasswd, even when the raw JSON response contains a "sed_passwd" key: systemAdvancedAPI has no field for it at all, so it is structurally impossible to decode into the model. |
| `TestSystemAdvancedAPI_NoSedPasswdField` | Unit | TestSystemAdvancedAPI_NoSedPasswdField verifies (via reflection) that systemAdvancedAPI has no sed_passwd field at all, so a response containing a "sed_passwd" key is structurally undecodable into it. |
| `TestSystemAdvancedDataSourceModel_MatchesSchema` | Unit | TestSystemAdvancedDataSourceModel_MatchesSchema verifies that every tfsdk tag on SystemAdvancedDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestSystemAdvancedDataSource_NoSedPasswdAttribute` | Unit | TestSystemAdvancedDataSource_NoSedPasswdAttribute verifies that the datasource schema itself has no sed_passwd attribute. |
| `TestSystemAdvancedSchema_ComputedOnlyPair` | Unit | TestSystemAdvancedSchema_ComputedOnlyPair verifies that anonstats_token and isolated_gpu_pci_ids are Computed-only (not Optional) and carry NO plan modifiers. |
| `TestSystemAdvancedSchema_IDIsComputed` | Unit | TestSystemAdvancedSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestSystemAdvancedSchema_SedPasswdIsWriteOnlySecret` | Unit | TestSystemAdvancedSchema_SedPasswdIsWriteOnlySecret verifies that sed_passwd is Optional+Sensitive ONLY: never Computed (there is no server value to compute from) and carrying no plan modifiers. |
| `TestSystemAdvancedSchema_WritableFieldsOptionalComputed` | Unit | TestSystemAdvancedSchema_WritableFieldsOptionalComputed spot-checks that writable fields across each type are Optional+Computed with a UseStateForUnknown plan modifier. |
| `TestUpdatePayload_AllKnownFieldsSent` | Unit | TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes every guarded field when its model value is known, including a non-zero overprovision and a non-empty syslogservers list. |
| `TestUpdatePayload_ComputedOnlyPairNeverSent` | Unit | TestUpdatePayload_ComputedOnlyPairNeverSent verifies that anonstats_token and isolated_gpu_pci_ids never appear in the update payload (they have no corresponding updatePayload field at all, so this documents the contract by construction). |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any field whose model value is null or unknown. |
| `TestUpdatePayload_OverprovisionThreeWay` | Unit | TestUpdatePayload_OverprovisionThreeWay verifies the three-way behavior for overprovision: null/unknown omits the key entirely, an explicit 0 sends JSON nil (clearing overprovision), and any other value sends that value. |
| `TestUpdatePayload_SedPasswdOnlyWhenSet` | Unit | TestUpdatePayload_SedPasswdOnlyWhenSet verifies that sed_passwd is omitted from the payload whenever its model value is null or unknown, and is included only when a value has actually been set. |
| `TestUpdatePayload_SyslogserversNilMapsToEmptyList` | Unit | TestUpdatePayload_SyslogserversNilMapsToEmptyList verifies that an explicitly-set-but-empty syslogservers list is sent as an empty slice (clearing the value on TrueNAS) rather than a nil ElementsAs result panicking or being omitted. |

## `internal/resources/system_dataset`  (14)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccSystemDatasetDataSource_basic` | Acceptance · conditional skip | TestAccSystemDatasetDataSource_basic reads the current TrueNAS system dataset configuration through the truenas_system_dataset datasource only. |
| `TestAccSystemDataset_basic` | Acceptance · conditional skip | TestAccSystemDataset_basic is intentionally skipped by default. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: SystemDatasetResource.Delete calls only this pure function. |
| `TestResponseToModel_MapsFields` | Unit | TestResponseToModel_MapsFields verifies that the API response maps through to the Terraform model unchanged, and that PoolExclude (write- only) is left untouched. |
| `TestSystemDatasetAPI_NoPoolExcludeField` | Unit | TestSystemDatasetAPI_NoPoolExcludeField verifies by construction that systemDatasetAPI has no pool_exclude field at all: this is a compile-time guarantee that pool_exclude can never be decoded from a systemdataset.config response. |
| `TestSystemDatasetDataSourceModel_MatchesSchema` | Unit | TestSystemDatasetDataSourceModel_MatchesSchema verifies that every tfsdk tag on SystemDatasetDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestSystemDatasetSchema_ComputedOnlyQuartet` | Unit | TestSystemDatasetSchema_ComputedOnlyQuartet verifies that basename, path, and pool_set are Computed-only with NO plan modifiers (they change whenever pool changes, so UseStateForUnknown would be wrong), while uuid is Computed-only WITH UseStateForUnknown. |
| `TestSystemDatasetSchema_IDIsComputed` | Unit | TestSystemDatasetSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestSystemDatasetSchema_PoolExcludeOptionalOnly` | Unit | TestSystemDatasetSchema_PoolExcludeOptionalOnly verifies that pool_exclude is Optional ONLY (never Computed): it is a write-only migration argument that TrueNAS never returns on systemdataset.config. |
| `TestSystemDatasetSchema_PoolOptionalComputed` | Unit | TestSystemDatasetSchema_PoolOptionalComputed verifies that pool is Optional+Computed with a UseStateForUnknown plan modifier. |
| `TestUpdatePayload_ComputedOnlyQuartetNeverSent` | Unit | TestUpdatePayload_ComputedOnlyQuartetNeverSent verifies that basename, path, uuid, and pool_set never appear in the update payload. |
| `TestUpdatePayload_PoolExcludeOnlyWhenSet` | Unit | TestUpdatePayload_PoolExcludeOnlyWhenSet verifies that pool_exclude is omitted from the update payload unless explicitly set. |
| `TestUpdatePayload_PoolGuarded` | Unit | TestUpdatePayload_PoolGuarded verifies that pool is omitted from the update payload when null/unknown, and included when known. |
| `TestUpdate_UsesCallJob` | Unit | TestUpdate_UsesCallJob verifies, by parsing this package's own source, that Update calls through applyUpdate (which wraps client.CallJob) rather than a plain synchronous client.Call for systemdataset.update. |

## `internal/resources/system_general`  (15)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccSystemGeneralDataSource_basic` | Acceptance · conditional skip | TestAccSystemGeneralDataSource_basic reads the current TrueNAS system general configuration through the truenas_system_general datasource only. |
| `TestAccSystemGeneral_basic` | Acceptance · conditional skip | TestAccSystemGeneral_basic is intentionally skipped by default. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: SystemGeneralResource.Delete calls only this pure function. |
| `TestResponseToModel_ListsNilMapToEmpty` | Unit | TestResponseToModel_ListsNilMapToEmpty verifies that nil list fields from the API map to empty (non-null) Terraform lists. |
| `TestResponseToModel_MapsPlainFields` | Unit | TestResponseToModel_MapsPlainFields verifies that non-nullable, non-pointer fields are copied through unchanged. |
| `TestResponseToModel_NullableFieldsMapToNull` | Unit | TestResponseToModel_NullableFieldsMapToNull verifies that a nil ui_certificate/usage_collection on the wire maps to Int64Null/BoolNull (not a zero value), and that a non-nil value maps through unchanged. |
| `TestSystemGeneralDataSourceModel_MatchesSchema` | Unit | TestSystemGeneralDataSourceModel_MatchesSchema verifies that every tfsdk tag on SystemGeneralDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestSystemGeneralSchema_ComputedOnlyTrio` | Unit | TestSystemGeneralSchema_ComputedOnlyTrio verifies that ui_certificate_name, usage_collection_is_set, and wizardshown are Computed-only (not Optional) with NO plan modifiers: ui_certificate_name is derived from ui_certificate and changes when the certificate is reassigned, so carrying prior state forward would trip Terraform's post-apply consistency check. |
| `TestSystemGeneralSchema_IDIsComputed` | Unit | TestSystemGeneralSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestSystemGeneralSchema_WritableFieldsOptionalComputed` | Unit | TestSystemGeneralSchema_WritableFieldsOptionalComputed verifies that every writable field is Optional+Computed with a UseStateForUnknown plan modifier. |
| `TestUICertificateDecode_bothShapes` | Unit | TestUICertificateDecode_bothShapes verifies uiCertificate decodes both wire shapes of system.general.config's ui_certificate: TrueNAS 26.0's bare integer ID and 25.10's full certificate object (id + name), plus null. |
| `TestUpdatePayload_AllKnownFieldsSent` | Unit | TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes every guarded field when its model value is known, including a non-zero ui_certificate. |
| `TestUpdatePayload_ComputedOnlyTrioNeverSent` | Unit | TestUpdatePayload_ComputedOnlyTrioNeverSent verifies that ui_certificate_name, usage_collection_is_set, and wizardshown never appear in the update payload, regardless of their model values (they have no corresponding updatePayload field at all, so this documents the contract by construction: attempting to reference them would be a compile error). |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any field whose model value is null or unknown. |
| `TestUpdatePayload_UICertificateThreeWay` | Unit | TestUpdatePayload_UICertificateThreeWay verifies the three-way behavior for ui_certificate: null/unknown omits the key entirely, an explicit 0 sends JSON nil (clearing the certificate), and any other value sends that value. |

## `internal/resources/tn_connect_config`  (26)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccTnConnectConfigDataSource_basic` | Acceptance · Tier 1 | TestAccTnConnectConfigDataSource_basic reads the current TrueNAS Connect configuration through the truenas_tn_connect_config datasource only. |
| `TestAccTnConnectConfig_setAndRestore` | Acceptance · conditional skip | TestAccTnConnectConfig_setAndRestore is intentionally skipped unconditionally. |
| `TestDeleteWarningDiagnostics` | Unit | --- deleteWarningDiagnostics -------------------------------------------- |
| `TestNeedsUpdateCall_DirectEmptyMap` | Unit |  |
| `TestNeedsUpdateCall_EmptyPayloadIsFalse` | Unit | --- needsUpdateCall -------------------------------------------------------  This is what makes "an empty-config Create/Update performs a read-only fetchConfig, never an unprobed tn_connect.update({})" independently testable without a live client: Create/Update call needsUpdateCall(payload) on the exact map updatePayload returns, and skip applyUpdate entirely when it reports false. |
| `TestNeedsUpdateCall_ExplicitFalseIsTrue` | Unit |  |
| `TestNeedsUpdateCall_ExplicitTrueIsTrue` | Unit |  |
| `TestNonNilStrings` | Unit | --- nonNilStrings ----------------------------------------------------------- |
| `TestRegistrationDetailsString_Empty` | Unit | --- registrationDetailsString -------------------------------------------- |
| `TestRegistrationDetailsString_Populated` | Unit |  |
| `TestResponseToDataSourceModel` | Unit |  |
| `TestResponseToModel_2510Shape` | Unit |  |
| `TestResponseToModel_260Shape` | Unit |  |
| `TestStringListOrNull_EmptySliceIsKnownEmptyList` | Unit |  |
| `TestStringListOrNull_NilPointerIsNullList` | Unit | --- stringListOrNull ---------------------------------------------------- |
| `TestStringListOrNull_PopulatedSlice` | Unit |  |
| `TestTnConnectConfigDataSourceModel_MatchesSchema` | Unit | TestTnConnectConfigDataSourceModel_MatchesSchema is the datasource counterpart of TestTnConnectConfigModel_MatchesSchema. |
| `TestTnConnectConfigModel_MatchesSchema` | Unit | TestTnConnectConfigModel_MatchesSchema verifies that every tfsdk tag on TnConnectConfigModel has a corresponding attribute in the resource schema, and vice versa. |
| `TestTnConnectConfigSchema_IDIsComputed` | Unit |  |
| `TestTnConnectConfigSchema_NoUnexpectedAttributes` | Unit |  |
| `TestTnConnectConfigSchema_OnlyEnabledIsWritable` | Unit | TestTnConnectConfigSchema_OnlyEnabledIsWritable is the schema-level half of this resource's safety contract: "enabled" must be the ONLY Optional+Computed (i.e. |
| `TestUpdatePayload_AllUnknownOmitsEnabled` | Unit | --- updatePayload ---------------------------------------------------------  This is the load-bearing safety-critical test group: no committed code path exercised by any test in this repository may ever cause tn_connect.update to be called with {"enabled": true}. |
| `TestUpdatePayload_ExplicitFalseIncluded` | Unit |  |
| `TestUpdatePayload_ExplicitTrueIncluded` | Unit |  |
| `TestUpdatePayload_NeverIncludesAnyOtherField` | Unit |  |
| `TestUpdatePayload_NullOmitsEnabled` | Unit |  |

## `internal/resources/truecommand_config`  (15)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccTrueCommandConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccTrueCommandConfigDataSource_basic reads the current TrueNAS TrueCommand configuration through the truenas_truecommand_config datasource only. |
| `TestAccTrueCommandConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccTrueCommandConfig_setAndRestore drives the singleton truenas_truecommand_config resource's "api_key" field through a schema-valid throwaway 16-character value, then restores the box's original api_key (read before the test ran) directly via acctest.RestoreCall in t.Cleanup, then imports it. |
| `TestResponseToDataSourceModel_FullShape` | Unit | TestResponseToDataSourceModel_FullShape mirrors TestResponseToModel_FullShape for the datasource model. |
| `TestResponseToModel_FullShape` | Unit | TestResponseToModel_FullShape verifies responseToModel against the field shape probed live (identical on TrueNAS 25.10.4 HA and 26.0): api_key round-trips verbatim (unmasked), nullable fields decode to true null. |
| `TestResponseToModel_NullableFieldsNull` | Unit | TestResponseToModel_NullableFieldsNull verifies api_key/remote_url/ remote_ip_address decode to true Terraform null (not empty string) when the API returns null, matching the probed disabled-state shape (both TrueNAS 25.10.4 HA and 26.0's live truecommand.config: {"api_key": null, "enabled": false, "remote_ip_address": null, "remote_url": null, ...}). |
| `TestTrueCommandConfigDataSourceModel_MatchesSchema` | Unit | TestTrueCommandConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on TrueCommandConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestTrueCommandConfigSchema_APIKeyIsSensitiveNotWriteOnly` | Unit | TestTrueCommandConfigSchema_APIKeyIsSensitiveNotWriteOnly verifies "api_key" is Sensitive but NOT WriteOnly: decisive live probe evidence (model.go's doc comment) confirmed truecommand.config returns it verbatim on read-back, unlike a genuinely masked/write-only credential. |
| `TestTrueCommandConfigSchema_ComputedOnlyFields` | Unit | TestTrueCommandConfigSchema_ComputedOnlyFields verifies the read-only status/connection fields are Computed-only. |
| `TestTrueCommandConfigSchema_EnabledIsOptionalComputed` | Unit | TestTrueCommandConfigSchema_EnabledIsOptionalComputed verifies "enabled" is the safety-critical Optional+Computed field, matching the tn_connect_config precedent. |
| `TestTrueCommandConfigSchema_IDIsComputed` | Unit | TestTrueCommandConfigSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestUpdatePayload_BothSet` | Unit | TestUpdatePayload_BothSet verifies both keys are included together when both are set. |
| `TestUpdatePayload_EmptyWhenUnset` | Unit | TestUpdatePayload_EmptyWhenUnset verifies updatePayload returns an empty map when both "enabled" and "api_key" are null/unknown — the state used whenever a caller manages this resource purely for its read-only status fields. |
| `TestUpdatePayload_IncludesAPIKeyWhenSet` | Unit | TestUpdatePayload_IncludesAPIKeyWhenSet verifies "api_key" is included (and only "api_key") when set and enabled is left unset — the decisive probe's exact payload shape (see model.go's doc comment). |
| `TestUpdatePayload_IncludesEnabledWhenSet` | Unit | TestUpdatePayload_IncludesEnabledWhenSet verifies "enabled" is included (and only "enabled") when set and api_key is left unset. |
| `TestUpdatePayload_NeverSendsEnabledTrue` | Unit | TestUpdatePayload_NeverSendsEnabledTrue is a safety-critical guard: no path through this function's logic can ever produce {"enabled": true} unless the model itself already carries Enabled=true — this is the negative-space check that the function does no implicit escalation (e.g. |

## `internal/resources/tunable`  (10)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccTunable_basic` | Acceptance · Tier 1 | TestAccTunable_basic tests create, update, and import of a sysctl tunable. |
| `TestDecodeCreateResult_BareIntShape` | Unit | TestDecodeCreateResult_BareIntShape verifies decodeCreateResult handles a job result that is a bare integer ID. |
| `TestDecodeCreateResult_Invalid` | Unit | TestDecodeCreateResult_Invalid verifies decodeCreateResult returns an error for unparseable input. |
| `TestDecodeCreateResult_ObjectShape` | Unit | TestDecodeCreateResult_ObjectShape verifies decodeCreateResult handles a job result that is the full created tunable object. |
| `TestResponseToModel` | Unit | TestResponseToModel verifies responseToModel populates all fields and never touches UpdateInitramfs (write-only). |
| `TestTunableCreatePayload` | Unit | TestTunableCreatePayload verifies createPayload always includes var/value and omits optionals that are unset. |
| `TestTunableCreatePayload_AllSet` | Unit | TestTunableCreatePayload_AllSet verifies optionals are included when known. |
| `TestTunableSchema` | Unit | TestTunableSchema verifies key schema attributes. |
| `TestTunableUpdatePayload` | Unit | TestTunableUpdatePayload verifies updatePayload always includes value and never includes var/type (immutable after create). |
| `TestTunableUpdatePayload_UpdateInitramfsGuarded` | Unit | TestTunableUpdatePayload_UpdateInitramfsGuarded verifies update_initramfs is included in updatePayload only when set. |

## `internal/resources/twofactor_auth`  (15)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccTwoFactorAuthDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccTwoFactorAuthDataSource_basic reads the current TrueNAS two-factor authentication configuration through the truenas_twofactor_auth datasource only. |
| `TestAccTwoFactorAuth_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccTwoFactorAuth_setAndRestore drives the singleton truenas_twofactor_auth resource's "window" field through a different valid value and back to the value read from the box before the test ran, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable, which matters here specifically because Delete must never disable 2FA. |
| `TestResponseToDataSourceModel` | Unit | TestResponseToDataSourceModel verifies the datasource mapping mirrors responseToModel. |
| `TestResponseToModel` | Unit | TestResponseToModel verifies responseToModel against the shape probed from a live auth.twofactor.config call (identical on TrueNAS 25.10 and 26.0). |
| `TestResponseToModel_EnabledAndSSHTrue` | Unit | TestResponseToModel_EnabledAndSSHTrue verifies the mapping also handles the "everything true" shape, matching what the SAFETY VERIFICATION probe observed after auth.twofactor.update({"enabled": true}). |
| `TestTwoFactorAuthDataSourceModel_MatchesSchema` | Unit | TestTwoFactorAuthDataSourceModel_MatchesSchema verifies that every tfsdk tag on TwoFactorAuthDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestTwoFactorAuthSchema_EnabledIsOptionalComputed` | Unit | TestTwoFactorAuthSchema_EnabledIsOptionalComputed verifies that "enabled" is Optional+Computed with a UseStateForUnknown plan modifier — settable, and (via updatePayload sourcing inclusion from req.Config rather than the plan) never sent unless explicitly configured, which is what keeps the committed acceptance test from ever touching it. |
| `TestTwoFactorAuthSchema_IDIsComputed` | Unit | TestTwoFactorAuthSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestTwoFactorAuthSchema_ServicesIsOptionalComputed` | Unit | TestTwoFactorAuthSchema_ServicesIsOptionalComputed verifies that "services" is an Optional+Computed SingleNestedAttribute with a nested Required "ssh" bool, matching the probed shape. |
| `TestTwoFactorAuthSchema_WindowIsOptionalComputed` | Unit | TestTwoFactorAuthSchema_WindowIsOptionalComputed verifies that "window" is Optional+Computed with a UseStateForUnknown plan modifier and an AtLeast(0) validator, matching the probed auth.twofactor.update schema (minimum 0, no explicit maximum). |
| `TestUpdatePayload_AllFieldsSet` | Unit | TestUpdatePayload_AllFieldsSet verifies that updatePayload includes every field with the exact keys observed in the auth.twofactor.update probe. |
| `TestUpdatePayload_EnabledExplicitlySet` | Unit | TestUpdatePayload_EnabledExplicitlySet verifies that a user who DOES set "enabled" in their HCL config still gets it included in the payload: the config-driven guard in updatePayload must not suppress explicitly-configured values, only unconfigured (null-in-config) ones. |
| `TestUpdatePayload_UnsetOptionalsOmitted` | Unit | TestUpdatePayload_UnsetOptionalsOmitted verifies that every field is omitted when null (the real-world req.Config shape for an attribute the user never set in HCL), so the current TrueNAS-side value is left unchanged rather than overwritten with a zero value. |
| `TestUpdatePayload_WindowOnly` | Unit | TestUpdatePayload_WindowOnly verifies the shape updatePayload actually sees on the real Create/Update path when the caller passes a model built from req.Config (as resource.go now does): the acceptance test's HCL sets only "window", so "enabled" and "services" are null in config — NOT Unknown. |

## `internal/resources/ups_config`  (24)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccUPSConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccUPSConfigDataSource_basic reads the current TrueNAS UPS configuration through the truenas_ups_config datasource only. |
| `TestAccUPSConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccUPSConfig_setAndRestore drives the singleton truenas_ups_config resource's "description" field (a cosmetic, low-risk descriptive string) through a test value and back to the value read from the box before the test ran, then imports it. |
| `TestBasePayloadFromConfig_IncludesAllWritableFields` | Unit | TestBasePayloadFromConfig_IncludesAllWritableFields verifies that basePayloadFromConfig carries every writable, non-secret field from a live upsConfigAPI response into the base payload, including nil ShutdownCmd/ NoCommWarnTime mapping to nil (not omitted) entries, and that complete_identifier/monpwd are never included. |
| `TestDeleteWarningDiagnostics` | Unit | TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder returns exactly one warning (no errors) and does not require or touch a client — this is what makes "Delete makes no client calls" verifiable: UPSConfigResource.Delete calls only this pure function. |
| `TestMergedPayload_PlanOverlaysLive` | Unit | TestMergedPayload_PlanOverlaysLive verifies that mergedPayload starts from the live config's fields and that any field the plan knows about overrides the live value, while fields the plan doesn't know about keep their live value — this is the fix for "ups_update.port: This field is required" / "ups_update.driver: This field is required" when a config sets only description. |
| `TestMergedPayload_SecretAbsentUnlessSet` | Unit | TestMergedPayload_SecretAbsentUnlessSet verifies that mergedPayload never includes "monpwd" unless the plan explicitly sets it: the base built from the live config has no secret fields, and updatePayload only contributes monpwd when it's known and non-null. |
| `TestResponseToDataSourceModel_NilPointersBecomeZeroValues` | Unit | TestResponseToDataSourceModel_NilPointersBecomeZeroValues mirrors the same nil-to-zero-value handling for the datasource model. |
| `TestResponseToModel_CompleteIdentifierSet` | Unit | TestResponseToModel_CompleteIdentifierSet verifies that complete_identifier is carried through from the API response as-is. |
| `TestResponseToModel_MonPwdNeverSet` | Unit | TestResponseToModel_MonPwdNeverSet verifies that responseToModel never writes to m.MonPwd, regardless of its prior value: monpwd is write-only and the API never returns a usable value for it. |
| `TestResponseToModel_NoCommWarnTimeNilBecomesZero` | Unit | TestResponseToModel_NoCommWarnTimeNilBecomesZero verifies that a nil nocommwarntime from the API maps to 0 in the model. |
| `TestResponseToModel_NoCommWarnTimeSet` | Unit | TestResponseToModel_NoCommWarnTimeSet verifies that a non-nil nocommwarntime from the API is carried through as-is. |
| `TestResponseToModel_ShutdownCmdNilBecomesEmptyString` | Unit | TestResponseToModel_ShutdownCmdNilBecomesEmptyString verifies that a nil shutdowncmd from the API maps to an empty string in the model. |
| `TestResponseToModel_ShutdownCmdSet` | Unit | TestResponseToModel_ShutdownCmdSet verifies that a non-nil shutdowncmd from the API is carried through as-is. |
| `TestUPSConfigDataSourceModel_MatchesSchema` | Unit | TestUPSConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on UPSConfigDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestUPSConfigDataSourceModel_NoMonPwdField` | Unit | TestUPSConfigDataSourceModel_NoMonPwdField verifies that UPSConfigDataSourceModel has no monpwd field: the API never returns a usable value for it, so a datasource attribute for it would always read as empty/unknown. |
| `TestUPSConfigSchema_CompleteIdentifierIsComputedOnly` | Unit | TestUPSConfigSchema_CompleteIdentifierIsComputedOnly verifies that "complete_identifier" is Computed-only with UseStateForUnknown plan modifiers (it is server-derived and should never be sent to ups.update). |
| `TestUPSConfigSchema_IDIsComputed` | Unit | TestUPSConfigSchema_IDIsComputed verifies that "id" is a Computed-only StringAttribute with UseStateForUnknown, since it's a fixed singleton value never supplied by the user. |
| `TestUPSConfigSchema_MonPwdIsSensitiveWriteOnly` | Unit | TestUPSConfigSchema_MonPwdIsSensitiveWriteOnly verifies that "monpwd" is Optional + Sensitive, and specifically NOT Computed (write-only: never read back from TrueNAS, so it must not participate in drift detection). |
| `TestUPSConfigSchema_OtherFieldsAreOptionalComputed` | Unit | TestUPSConfigSchema_OtherFieldsAreOptionalComputed verifies that every non-secret, non-id, non-complete_identifier field is Optional+Computed with UseStateForUnknown plan modifiers. |
| `TestUpdatePayload_CompleteIdentifierNeverSent` | Unit | TestUpdatePayload_CompleteIdentifierNeverSent verifies that complete_identifier is never included in the update payload, regardless of its model value: it is server-derived and read-only. |
| `TestUpdatePayload_MonPwdOnlyWhenSet` | Unit | TestUpdatePayload_MonPwdOnlyWhenSet verifies that monpwd is included only when it has a known, non-null value, and is otherwise omitted entirely (never sent as an empty string or null). |
| `TestUpdatePayload_NoCommWarnTimeThreeWay` | Unit | TestUpdatePayload_NoCommWarnTimeThreeWay verifies the three-way nullable handling for nocommwarntime: omitted when null/unknown, sent as nil when explicitly set to 0, and sent as the value otherwise. |
| `TestUpdatePayload_OnlyKnownFieldsSent` | Unit | TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any field whose model value is null or unknown, for every guarded non-nullable, non-secret field. |
| `TestUpdatePayload_ShutdownCmdThreeWay` | Unit | TestUpdatePayload_ShutdownCmdThreeWay verifies the three-way nullable handling for shutdowncmd: omitted when null/unknown, sent as nil when explicitly set to "", and sent as the value otherwise. |

## `internal/resources/user`  (12)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccUser_basic` | Acceptance · Tier 1 | TestAccUser_basic creates a local user with a password set, checks its attributes, updates full_name and shell in place, imports it by its numeric id (ignoring the write-only password), and verifies destruction. |
| `TestResponseToModel` | Unit | TestResponseToModel verifies responseToModel populates all fields correctly. |
| `TestResponseToModel_GroupBareInt` | Unit | TestResponseToModel_GroupBareInt verifies responseToModel decodes a "group" field returned as a bare integer id. |
| `TestResponseToModel_GroupNull` | Unit | TestResponseToModel_GroupNull verifies responseToModel maps a null/absent "group" field to Int64Null rather than erroring or defaulting to 0. |
| `TestResponseToModel_GroupObjectDecode` | Unit | TestResponseToModel_GroupObjectDecode verifies responseToModel decodes a "group" field embedded as an object ({"id": N, ...}), matching the live user.query/get_instance wire shape. |
| `TestResponseToModel_NilPointers` | Unit | TestResponseToModel_NilPointers verifies nil email/sshpubkey map to empty strings. |
| `TestUserCreatePayload` | Unit | TestUserCreatePayload verifies that createPayload includes uid and username. |
| `TestUserCreatePayload_GroupCreate` | Unit | TestUserCreatePayload_GroupCreate verifies that createPayload includes group_create when set, and omits it (and 'group') when neither is set. |
| `TestUserCreatePayload_GroupID` | Unit | TestUserCreatePayload_GroupID verifies that createPayload includes 'group' when a primary group id is set, and omits 'group_create' when it is unset. |
| `TestUserSchema` | Unit | TestUserSchema verifies key schema attributes. |
| `TestUserUpdatePayload` | Unit | TestUserUpdatePayload verifies that updatePayload omits uid and username. |
| `TestUserUpdatePayload_GroupIncluded` | Unit | TestUserUpdatePayload_GroupIncluded verifies that updatePayload includes 'group' when set (primary group is updatable, unlike uid/username), and never includes 'group_create' (create-only, write-only). |

## `internal/resources/vm`  (11)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccVM_basic` | Acceptance · Tier 1 | TestAccVM_basic tests create, update, and import of a VM. |
| `TestResponseToModel_AllFields` | Unit | TestResponseToModel_AllFields verifies that all remaining fields are mapped correctly from vmAPI to VMModel. |
| `TestResponseToModel_NilHandling` | Unit | TestResponseToModel_NilHandling verifies that a nil MinMemory maps to 0 and a nil CPUModel maps to "". |
| `TestResponseToModel_NonNilHandling` | Unit | TestResponseToModel_NonNilHandling verifies that non-nil MinMemory/CPUModel pointers are dereferenced correctly. |
| `TestResponseToModel_RunningDerivedFromStatus` | Unit | TestResponseToModel_RunningDerivedFromStatus verifies that Running is derived from Status.State == "RUNNING". |
| `TestVMApiPayload_CPUModelEmptyOmitted` | Unit | TestVMApiPayload_CPUModelEmptyOmitted verifies that cpu_model is omitted from the payload when it is explicitly set to "". |
| `TestVMApiPayload_IncludesSetOptionals` | Unit | TestVMApiPayload_IncludesSetOptionals verifies that apiPayload includes keys for optional fields that are explicitly set (including zero values like vcpus: 0, since VCPUs/Cores/Threads have no special zero-omission rule). |
| `TestVMApiPayload_MinMemoryZeroOmitted` | Unit | TestVMApiPayload_MinMemoryZeroOmitted verifies that min_memory is omitted from the payload when it is explicitly set to 0. |
| `TestVMApiPayload_OmitsUnsetOptionals` | Unit | TestVMApiPayload_OmitsUnsetOptionals verifies that apiPayload does not send keys for null/unknown optional fields (e.g. |
| `TestVMSchema` | Unit | TestVMSchema verifies that the resource schema has the expected attributes and that key attributes have the correct types/behaviors. |
| `TestVMSchema_OptionalComputedHavePlanModifiers` | Unit | TestVMSchema_OptionalComputedHavePlanModifiers verifies that all Optional+Computed attributes (except status) have at least one plan modifier. |

## `internal/resources/vm_device`  (15)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccVMDevice_basic` | Acceptance · Tier 1 | TestAccVMDevice_basic tests create, update, and import of a VM device (a DISPLAY device attached to its own RandName-suffixed, non-running, non-autostart VM fixture). |
| `TestAttributesDrifted_APIAddedKey` | Unit | TestAttributesDrifted_APIAddedKey verifies that a key present only in the API response (a server-added default) is NOT considered drift. |
| `TestAttributesDrifted_ChangedValue` | Unit | TestAttributesDrifted_ChangedValue verifies that a changed value for a user-set key is reported as drift. |
| `TestAttributesDrifted_Identical` | Unit | TestAttributesDrifted_Identical verifies that identical maps are not considered drifted. |
| `TestAttributesDrifted_MissingUserKey` | Unit | TestAttributesDrifted_MissingUserKey verifies that a user-set key absent from the API response is considered drift. |
| `TestAttributesMap_InvalidJSON` | Unit | TestAttributesMap_InvalidJSON verifies that attributesMap rejects malformed JSON with an error diagnostic. |
| `TestAttributesMap_MissingDtype` | Unit | TestAttributesMap_MissingDtype verifies that attributesMap rejects valid JSON that lacks the required "dtype" key. |
| `TestAttributesMap_Valid` | Unit | TestAttributesMap_Valid verifies that attributesMap parses valid JSON with a dtype key without error. |
| `TestCreatePayload_IncludesVM` | Unit | TestCreatePayload_IncludesVM verifies createPayload includes vm and attributes. |
| `TestSchema_AttributesRequired` | Unit | TestSchema_AttributesRequired verifies that "attributes" is a Required StringAttribute, and that it is Sensitive: the blob can carry secrets for some device types (e.g. |
| `TestSchema_IDComputedUseStateForUnknown` | Unit | TestSchema_IDComputedUseStateForUnknown verifies that the "id" attribute is Computed with a UseStateForUnknown plan modifier. |
| `TestSchema_OrderOptionalComputed` | Unit | TestSchema_OrderOptionalComputed verifies that "order" is Optional+Computed with a UseStateForUnknown plan modifier. |
| `TestSchema_VMRequiresReplace` | Unit | TestSchema_VMRequiresReplace verifies that the "vm" attribute is Required and has a RequiresReplace plan modifier. |
| `TestUpdatePayload_OmitsOrderWhenUnset` | Unit | TestUpdatePayload_OmitsOrderWhenUnset verifies order is left out when null/unknown. |
| `TestUpdatePayload_OmitsVM` | Unit | TestUpdatePayload_OmitsVM verifies that updatePayload contains attributes (and order when set) but never the "vm" key, since a device cannot be moved between VMs. |

## `internal/resources/vmware`  (11)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccVMware_basic` | Acceptance · conditional skip | TestAccVMware_basic is intentionally skipped unconditionally. |
| `TestApiPayload_AllFieldsAlwaysIncluded` | Unit | TestApiPayload_AllFieldsAlwaysIncluded verifies apiPayload includes all five keys every time: every field is Required in the schema (no Optional/Computed fields at all, unlike cloud_backup/app_registry), so there is no "omit unless set" branch to test. |
| `TestApiPayload_UsesConfigPasswordNotModelPassword` | Unit | TestApiPayload_UsesConfigPasswordNotModelPassword verifies apiPayload always uses the explicitly-passed cfgPassword argument, not m.Password — the write-only safety contract (m.Password may be null/unknown from a plan where the framework has nulled the WriteOnly attribute). |
| `TestResponseToDataSourceModel_FullShape` | Unit | TestResponseToDataSourceModel_FullShape mirrors TestResponseToModel_FullShape for the datasource model. |
| `TestResponseToModel_FullShape` | Unit | TestResponseToModel_FullShape verifies responseToModel against the field shape probed live via core.get_methods (identical on both TrueNAS 25.10.4 HA and 26.0) and does NOT touch Password. |
| `TestResponseToModel_StateNullableFieldsNull` | Unit | TestResponseToModel_StateNullableFieldsNull verifies state.error/ state.datetime decode to true Terraform null (not empty string) when the API omits them, matching the probed schema's individually-optional sub-fields. |
| `TestVMwareDataSourceModel_MatchesSchema` | Unit | TestVMwareDataSourceModel_MatchesSchema verifies that every tfsdk tag on VMwareDataSourceModel has a corresponding attribute in the datasource schema, and vice versa. |
| `TestVMwareSchema_IDIsComputed` | Unit | TestVMwareSchema_IDIsComputed verifies "id" is Computed-only. |
| `TestVMwareSchema_PasswordIsSensitiveWriteOnly` | Unit | TestVMwareSchema_PasswordIsSensitiveWriteOnly verifies "password" is marked both Sensitive and WriteOnly: no live vmware entry was ever obtainable to observe a read-back value (vmware.create validates against the real endpoint before persisting anything — see schema.go's description), so this mirrors truenas_app_registry's identically-situated "password" rather than truenas_cloud_backup's (which DOES have live read-back evidence). |
| `TestVMwareSchema_RequiredFields` | Unit | TestVMwareSchema_RequiredFields verifies "datastore", "filesystem", "hostname", "username", "password" are Required, matching vmware.create's own "required" list (probed via core.get_methods, identical on both probed releases). |
| `TestVMwareSchema_StateIsNestedComputed` | Unit | TestVMwareSchema_StateIsNestedComputed verifies the nested "state" attribute is Computed-only with its three documented sub-fields. |

## `internal/resources/webshare`  (17)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccWebshare_basic` | Acceptance · conditional skip | TestAccWebshare_basic creates a dataset fixture and a Webshare share on its mountpoint, checks its attributes, updates "enabled" in place (this resource's equivalent of smb/nfs's "comment" cosmetic update field — sharing.webshare has no comment field at all, probed live), imports it by its numeric id, and verifies destruction of both the share and the dataset fixture. |
| `TestCreatePayload` | Unit | --- createPayload / updatePayload -------------------------------------------- |
| `TestResponseToDataSourceModel` | Unit |  |
| `TestResponseToModel` | Unit |  |
| `TestResponseToModel_NilPointersBecomeNull` | Unit |  |
| `TestUpdatePayload` | Unit |  |
| `TestVersionGateDiagnostics_AtOrAboveFloor` | Unit |  |
| `TestVersionGateDiagnostics_BelowFloor` | Unit | --- versionGateDiagnostics ------------------------------------------------ |
| `TestWebshareDataSourceModel_MatchesSchema` | Unit | TestWebshareDataSourceModel_MatchesSchema is the datasource counterpart of TestWebshareModel_MatchesSchema. |
| `TestWebshareModel_MatchesSchema` | Unit | TestWebshareModel_MatchesSchema verifies that every tfsdk tag on WebshareModel has a corresponding attribute in the resource schema, and vice versa. |
| `TestWebshareSchema_ComputedOnlyFields` | Unit |  |
| `TestWebshareSchema_EnabledAndIsHomeBaseHaveDefaults` | Unit |  |
| `TestWebshareSchema_IDIsComputed` | Unit |  |
| `TestWebshareSchema_NameIsRequiredAndMutable` | Unit |  |
| `TestWebshareSchema_NoCommentAttribute` | Unit |  |
| `TestWebshareSchema_NoUnexpectedAttributes` | Unit |  |
| `TestWebshareSchema_PathIsRequiredAndForceNew` | Unit |  |

## `internal/resources/webshare_config`  (20)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccWebshareConfigDataSource_basic` | Acceptance · Tier 2 (disruptive) | TestAccWebshareConfigDataSource_basic reads the current TrueNAS Webshare configuration through the truenas_webshare_config datasource only. |
| `TestAccWebshareConfig_setAndRestore` | Acceptance · Tier 2 (disruptive) | TestAccWebshareConfig_setAndRestore drives the singleton truenas_webshare_config resource's "search" field (a cosmetic, low-risk toggle — probed live: unlike passkey/groups/bindip it carries no authentication or network-exposure risk) through its opposite value, then back to the value read from the box before the test ran, then imports it. |
| `TestDeleteWarningDiagnostics` | Unit | --- deleteWarningDiagnostics -------------------------------------------- |
| `TestNonNilStrings` | Unit | --- nonNilStrings ----------------------------------------------------------- |
| `TestResponseToDataSourceModel` | Unit |  |
| `TestResponseToModel` | Unit |  |
| `TestResponseToModel_EmptyArraysBecomeKnownEmptyLists` | Unit |  |
| `TestUpdatePayload_AllUnknownOmitsEverything` | Unit | --- updatePayload -------------------------------------------------------  updatePayload's CALLERS MUST contract (see model.go) requires callers to invoke it on a model populated from req.Config, never req.Plan: for an Optional+Computed field the user never set in HCL, req.Config leaves it null, while req.Plan (via UseStateForUnknown) echoes the prior state's value. |
| `TestUpdatePayload_KnownFieldsIncluded` | Unit |  |
| `TestUpdatePayload_NullListsSendEmptySlice` | Unit |  |
| `TestUpdatePayload_PasskeyExplicitlySet` | Unit | TestUpdatePayload_PasskeyExplicitlySet verifies that a user who DOES set "passkey" in their HCL config still gets it included in the payload: the config-driven guard in updatePayload must not suppress explicitly-configured values, only unconfigured (null-in-config) ones. |
| `TestUpdatePayload_SearchOnly` | Unit | TestUpdatePayload_SearchOnly verifies the shape updatePayload actually sees on the real Create/Update path when the caller passes a model built from req.Config: an HCL config that sets only "search" leaves "bindip", "passkey", and "groups" null in config — NOT Unknown (Unknown never occurs in req.Config; Terraform resolves config to either a concrete value or null before the provider ever sees it). |
| `TestUpdatePayload_UnsetOptionalsOmitted` | Unit | TestUpdatePayload_UnsetOptionalsOmitted verifies that every field is omitted when null (the real-world req.Config shape for an attribute the user never set in HCL), so the current TrueNAS-side value is left unchanged rather than overwritten with a zero value. |
| `TestVersionGateDiagnostics_AtOrAboveFloor` | Unit |  |
| `TestVersionGateDiagnostics_BelowFloor` | Unit | --- versionGateDiagnostics ------------------------------------------------ |
| `TestWebshareConfigDataSourceModel_MatchesSchema` | Unit | TestWebshareConfigDataSourceModel_MatchesSchema is the datasource counterpart of TestWebshareConfigModel_MatchesSchema. |
| `TestWebshareConfigModel_MatchesSchema` | Unit | TestWebshareConfigModel_MatchesSchema verifies that every tfsdk tag on WebshareConfigModel has a corresponding attribute in the resource schema, and vice versa. |
| `TestWebshareConfigSchema_IDIsComputed` | Unit |  |
| `TestWebshareConfigSchema_NoUnexpectedAttributes` | Unit |  |
| `TestWebshareConfigSchema_OptionalComputedFields` | Unit |  |

## `internal/resources/zvol`  (3)

| Test | Tier / gate | Covers |
|---|---|---|
| `TestAccZvol_basic` | Acceptance · Tier 1 | TestAccZvol_basic creates a 64M zvol, checks its attributes, resizes it up to 128M in place, imports it by name, and verifies destruction. |
| `TestZvolAPIPayload` | Unit |  |
| `TestZvolSchema` | Unit |  |

