# Hunt tooling

Scripts that make the full acceptance suite runnable against the lab's
disposable TrueNAS boxes with **zero avoidable skips**, plus the probes used
in the adversarial hunts (old-state upgrade, enum round-trip, version drift).
Everything here reads box addresses, pools and credentials from arguments or
git-ignored env files; nothing box- or session-specific is hardcoded.
Fixture names and ports are documented defaults and can be overridden.

Every script prints its usage with `-h`. All of them assume **root ssh** to
the box and a **disposable** target: they set the disruptive gates, edit the
config database to restore state, and delete leftovers.

## Layout

| Script | Where it runs | What it does |
|---|---|---|
| `run-box.sh <ip> <log> <go test args>` | workstation | the runner: sources every env file for the box, sets all opt-in gates, runs `go test`, prints the SKIPPED list, exits 3 on an unexpected skip |
| `skips-report.sh <logs...>` | workstation | one line per distinct skipped test + reason across many logs |
| `build-ro-box.sh <ip>` | workstation | copies `registry-setup.sh` + `ro-fixtures.sh` to the box, runs them, writes `env.ro.<ip>.sh` |
| `registry-setup.sh` | on the box | two basic-auth `registry:2` containers (:9322 fixture, :9323 tests) |
| `ro-fixtures.sh [keytab_b64]` | on the box | READONLY_ADMIN user `tfhuntro` + API key `tf-hunt-ro` + one secret-bearing `tfhuntro-*` object per redaction data source |
| `dump-methods.sh <ip>...` | workstation | `core.get_methods` dump per box, input for the enum sweep and drift report |
| `upgrade/build-versions.sh [tag...]` | workstation | builds old release tags (git worktrees) and the current tree into `<work>/upgrade/bin/<version>/` |
| `upgrade/run-upgrade.sh <old> <ip>` | workstation | apply `upgrade/main.tf` with the old build, plan with the current build; a non-empty plan or an error is a finding |
| `enum/enum-sweep.py ...` | workstation | apply every schema enum value of an attribute, re-plan, report diffs |
| `schema/drift.py <old.json> <new.json>` | workstation | create/update fields dropped or added between two TrueNAS versions, cross-checked against the provider source |
| `leftovers.sh <ip>` | workstation (ssh) | list `tf-acc`/`tfhunt` leftovers that are not the intended fixtures |
| `box-state.sh <ip>...` | workstation (ssh) | leftovers + restore state (sshd port, 2FA, FTP TLS, UPS driver, boot environments) |
| `clean-hunt-vms.sh <ip>` | workstation (ssh) | delete stranded `tfhuntvmd*` VMs, keep `tfhuntrovm` |
| `restore-ups.sh <ip>` | workstation (ssh) | reset UPS driver/port to unconfigured via the config DB |
| `restore-ssh.sh <ip>...` | workstation (ssh) | restore `ssh.config` from the box's daily config snapshot when a test stranded sshd on 2222 |
| `lib.sh` | sourced | repo root, env dir, work dir, help, env sourcing |

## Env files

Directory: `$TFHUNT_ENV_DIR`, default `<repo>/.hunt-env/` (git-ignored). Each
file is plain `export VAR=value` lines. API keys live only here. Create them
with `chmod 600`.

| File | Required | Contents |
|---|---|---|
| `env.<ip>.sh` | yes (unless `TRUENAS_ENDPOINT` is already exported) | `TRUENAS_ENDPOINT=wss://<ip>/api/current`, `TRUENAS_API_KEY`, `TRUENAS_USERNAME=root`, `TRUENAS_TEST_POOL` (`tank`, or `fast` on the 28.0 box: `space` cannot do dataset encryption) |
| `env.s3.sh` | for `cloud_backup` | `TF_ACC_S3_ENDPOINT` (a throwaway minio, e.g. `http://<box>:9321`; `:9000` is 28.0's native s3d), `TF_ACC_S3_ACCESS`, `TF_ACC_S3_SECRET`, `TF_ACC_S3_BUCKET` |
| `env.keytab.sh` | for `kerberos_keytab` | `TRUENAS_DS_KEYTAB_B64`: `base64 -w0` of a keytab exported from the lab AD DC (`samba-tool domain exportkeytab /tmp/x.keytab --principal=<throwaway>@<REALM>`) |
| `env.registry.sh` | for `app_registry` | `TRUENAS_TEST_REGISTRY_URI=http://127.0.0.1:9323/`, `TRUENAS_TEST_REGISTRY_USERNAME`, `TRUENAS_TEST_REGISTRY_PASSWORD`, `TRUENAS_TEST_REGISTRY_USERNAME2`, `TRUENAS_TEST_REGISTRY_PASSWORD2`, `TFHUNT_REG_URI` (same as the URI); must match what `registry-setup.sh` was given |
| `env.ro.<ip>.sh` | for the redaction tests | written by `build-ro-box.sh`: `TRUENAS_RO_API_KEY`, `TRUENAS_RO_USERNAME`, `TRUENAS_RO_KEYTAB`, `TRUENAS_RO_REGISTRY`, `TFHUNT_REG_ID`, `TRUENAS_RO_ACME`, `TRUENAS_RO_ALERT`, `TRUENAS_RO_CLOUDSYNC`, `TRUENAS_RO_VMDEVICE` |

Per-box extras go in `env.<ip>.sh` too: on a box with spare blank disks,
`TRUENAS_TEST_DISKS=sdc,sdd,...` and `TRUENAS_TEST_SYSTEM_DATASET=1` enable
the pool-create and system-dataset migration tests.

Generated files (builds, terraform runs, schema dumps, logs) go under
`$TFHUNT_WORK_DIR`, default `<repo>/.hunt-work/` (git-ignored).

## Variables the scripts set or need

Set by `run-box.sh` on every run (the box must be disposable):
`TF_ACC=1 TRUENAS_DISRUPTIVE=1 TRUENAS_TEST_SSH_CONFIG=1 TRUENAS_TEST_SYSTEM_GENERAL=1 TRUENAS_TEST_TRUECOMMAND_ENABLE=1`.
With `RO=1` it also sets `TRUENAS_API_KEY=$TRUENAS_RO_API_KEY`,
`TRUENAS_USERNAME=$TRUENAS_RO_USERNAME`,
`TRUENAS_TEST_READONLY_REGISTRY_NAME=$TRUENAS_RO_REGISTRY` and
`TRUENAS_TEST_READONLY_REGISTRY_ID=$TFHUNT_REG_ID` so the whole provider runs
as the READONLY_ADMIN key.

Fixture-builder inputs (never stored in the repo; pass them in the
environment of `build-ro-box.sh`):

| Variable | Default | Used by |
|---|---|---|
| `TFHUNT_RO_PASSWORD` | required | `ro-fixtures.sh`: login password of the `tfhuntro` user |
| `TFHUNT_REG_PASSWORD` | required | `registry-setup.sh` htpasswd + `ro-fixtures.sh` `app.registry` entry |
| `TFHUNT_REG_PASSWORD2` | required | `registry-setup.sh`: second account for `password_wo` rotation tests |
| `TFHUNT_REG_USER` / `TFHUNT_REG_USER2` | `tfhuntreg` / `tfhuntreg2` | both |
| `TFHUNT_REG_PORT` / `TFHUNT_REG_PORT2` | `9322` / `9323` | both; 9322 holds the persistent fixture entry, 9323 is for tests (TrueNAS requires unique `app.registry` URIs) |
| `TFHUNT_ENV_DIR` | `<repo>/.hunt-env` | every workstation script |
| `TFHUNT_WORK_DIR` | `<repo>/.hunt-work` | builds, runs, dumps |
| `TFHUNT_SSH_PORT` | `2222` | `restore-ssh.sh`: port sshd is stranded on |
| `TFHUNT_UPGRADE_TF` | `upgrade/main.tf` | `run-upgrade.sh`: alternative probe config |

## Typical sequence for a new box

```
scripts/hunt/build-ro-box.sh 192.0.2.68          # registries + RO fixtures, writes env.ro.<ip>.sh
scripts/hunt/run-box.sh 192.0.2.68 .hunt-work/all-68.log ./internal/...
RO=1 scripts/hunt/run-box.sh 192.0.2.68 .hunt-work/ro-68.log ./internal/resources/app_registry/ -run readonly
scripts/hunt/skips-report.sh .hunt-work/all-*.log
scripts/hunt/box-state.sh 192.0.2.49 192.0.2.68 192.0.2.70
```

Upgrade probe and schema tools:

```
scripts/hunt/upgrade/build-versions.sh v1.5.24 v1.6.0
scripts/hunt/upgrade/run-upgrade.sh v1.6.0 192.0.2.68
scripts/hunt/dump-methods.sh 192.0.2.49 192.0.2.70
scripts/hunt/schema/drift.py .hunt-work/schema/methods-192.0.2.49.json .hunt-work/schema/methods-192.0.2.70.json
scripts/hunt/enum/enum-sweep.py 192.0.2.70 pool.dataset.create - truenas_dataset scripts/hunt/enum/dataset-base.tf '{"compression":"compression","sync":"sync"}'
```

## The SKIPPED list and accepted dead ends

`run-box.sh` ends every run with one line per skipped test: the test name
and the reason the test printed. Each line is tagged either
`[accepted dead end]` or `[UNBUILT FIXTURE / STALE GATE]`; any line in the
second group makes the run exit 3. A skip is only accepted when its reason
matches this pattern:

```
failover|vmware|enclosure|tn_connect|TrueNASConnect|Enterprise|ds_auth|container|not available on this release|requires TrueNAS|accepted from TrueNAS|below TrueNAS|needs TrueNAS <|from TrueNAS [0-9]
```

Why each is a dead end in this lab (the lab cannot supply the feature; a
skip is the correct outcome, not something to build a fixture for):

| Pattern | Why |
|---|---|
| `failover` | HA failover needs a licensed Enterprise HA pair; the lab boxes are single VMs. `failover_config` also has a test that needs a system NOT licensed for HA, which skips on an HA box. |
| `vmware` | `vmware.create` validates host/user/password against a real vCenter/ESXi; none exists in the lab |
| `enclosure` | `enclosure2.query` reports no enclosures on any lab box (enclosure and enclosure_label need SES hardware) |
| `tn_connect`, `TrueNASConnect` | TrueNAS Connect enrollment is a cloud account per box; `tn_connect.update` accepts only `enabled` on an enrolled box and the runtime guard skips on enrolled boxes |
| `Enterprise` | Enterprise-licensed features the lab has no license for (failover, enclosure, Enterprise-only system_general fields) |
| `ds_auth` | `system_general.ds_auth` requires directory services joined (and an Enterprise license on some releases); the DS lab is only joined during the DS test tier |
| `container` | the `container.*` / `lxc` / `webshare` namespaces are absent on 25.10; the same tests run on 26.0+ |
| `not available on this release`, `requires TrueNAS`, `accepted from TrueNAS`, `below TrueNAS`, `needs TrueNAS <`, `from TrueNAS N` | version gates: the test exercises a field or value one release does not accept (e.g. dataset `encryption_algorithm` below 26.0, GZIP-2..8 from 28.0, user import shape below 26.0). It is a dead end on the box that skips and must pass on another box in the cross-version run |

Anything else that skips is an unbuilt fixture (missing env, missing object
on the box) or a stale gate (a `t.Skip` whose condition no longer holds) and
must be fixed, not accepted. Never add to the pattern to make a run green;
add a fixture.

## Fixture defaults

| Fixture | Name / value | Where |
|---|---|---|
| READONLY_ADMIN user | `tfhuntro` in `truenas_readonly_administrators` | every box |
| READONLY_ADMIN API key | `tf-hunt-ro` (value shown once; `api_key.update {"reset":true}` re-mints) | every box |
| Redaction objects | `tfhuntro-keytab`, `tfhuntro-registry`, `tfhuntro-acme`, `tfhuntro-alert`, `tfhuntro-s3`, VM `tfhuntrovm` with a DISPLAY device on ports 5951/5952 | every box |
| Docker registries | containers `tfhunt-registry` (:9322) and `tfhunt-registry2` (:9323), data under `/mnt/<docker pool>/tfhunt-registry` | every box; `app.registry.create` does a real `docker login` and docker speaks plain HTTP only to loopback |
| S3 | throwaway minio on one box, port 9321, bucket `tfhuntbucket`; make buckets with a SigV4 PUT (`cloudsync.create_bucket` refuses type S3) | one box |
| UPS | boxes without a real UPS need `ups.update {"driver":"blazer_usb$625L","port":"auto"}` before the ups tests; `restore-ups.sh` afterwards | as needed |
| FTP TLS | certificate id 1 (`truenas_default`) | every box |
| Docker pool | must be configured (`docker.update pool=...`) so `docker_network` and the registries run | every box |

## Gotchas

- Run packages sequentially (`-p 1`, set by the runner): TrueNAS rate-limits
  logins to about 20/min.
- A failed `vm_device` test with `ImportStatePersist` leaves `tfhuntvmd*`
  VMs: `clean-hunt-vms.sh`.
- Stale `iscsi.auth` rows with `discovery_auth: BOGUS` (enum-sweep leftovers)
  break the CHAP_MUTUAL tests; delete them.
- `ssh_config` cleanup that echoes read-only host-key fields into
  `ssh.update` strands sshd on 2222: `restore-ssh.sh`.
- The post-test destroy in terraform-plugin-testing does not refresh state;
  boot-environment and other state-reading Delete guards can see stale
  values after out-of-band changes.
- Keytab: `ro-fixtures.sh` gets the keytab as a command-line argument on the
  box, and `build-ro-box.sh` passes the fixture passwords in the remote
  command line. Both show in `ps` on the box for the duration. Fine for
  disposable lab boxes; do not point these at anything else.
