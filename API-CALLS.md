# TrueNAS API Call Inventory

Every JSON-RPC method this provider invokes, extracted from the source by `scripts/list-api-calls.py`. Regenerate with `make api-calls`. Use it to catch upstream TrueNAS API changes that affect the provider: diff this file against a new release's method set.

**348** distinct methods across **55** namespaces (339 reached from non-test code).

## API version covered

- **Endpoint:** `/api/current` — JSON-RPC 2.0 over WebSocket (the rolling API alias; not a pinned schema version).
- **Baseline release:** TrueNAS **25.04** — the oldest release the provider targets. Methods without a `Since` mark below exist at this baseline.
- **Verified against:** TrueNAS **25.10** and **26.0** (live acceptance-tested).
- **26.0-only methods:** 22 methods require TrueNAS **26.0** or later (`Since` = 26.0 below): the `container`, `lxc`, `webshare`, and `sharing.webshare` namespaces do not exist earlier. SCRAM-SHA-512 API-key auth is also 26.0+ (a login mechanism, not a distinct method).

To catch API changes, diff the method set below against a target release's `core.get_methods` output. A method that moves from present to absent (or changes namespace) breaks the provider on that release.

Variant legend: `Call` = write/action · `CallJob` = job (async, polled) · `CallRead` = read/query. `Since` = oldest release exposing the method (blank = baseline 25.04).

| Method | Variant(s) | Since | Prod | Test-only |
|---|---|:--:|:--:|:--:|
| `acme.dns.authenticator.create` | Call |  | yes |  |
| `acme.dns.authenticator.delete` | Call |  | yes |  |
| `acme.dns.authenticator.get_instance` | CallRead |  | yes |  |
| `acme.dns.authenticator.query` | Call CallRead |  | yes |  |
| `acme.dns.authenticator.update` | Call |  | yes |  |
| `alertclasses.config` | Call CallRead |  | yes |  |
| `alertclasses.update` | Call |  | yes |  |
| `alertservice.create` | Call |  | yes |  |
| `alertservice.delete` | Call |  | yes |  |
| `alertservice.get_instance` | CallRead |  | yes |  |
| `alertservice.query` | Call CallRead |  | yes |  |
| `alertservice.update` | Call |  | yes |  |
| `api_key.create` | Call |  | yes |  |
| `api_key.delete` | Call |  | yes |  |
| `api_key.get_instance` | CallRead |  | yes |  |
| `api_key.query` | Call CallRead |  | yes |  |
| `api_key.update` | Call |  | yes |  |
| `app.create` | CallJob |  | yes |  |
| `app.delete` | CallJob |  | yes |  |
| `app.get_instance` | CallRead |  | yes |  |
| `app.query` | Call CallRead |  | yes |  |
| `app.registry.create` | Call |  | yes |  |
| `app.registry.delete` | Call |  | yes |  |
| `app.registry.get_instance` | CallRead |  | yes |  |
| `app.registry.query` | CallRead |  | yes |  |
| `app.registry.update` | Call |  | yes |  |
| `app.start` | CallJob |  | yes |  |
| `app.stop` | CallJob |  | yes |  |
| `app.update` | CallJob |  | yes |  |
| `app.upgrade` | CallJob |  | yes |  |
| `audit.config` | CallRead |  | yes |  |
| `audit.update` | Call |  | yes |  |
| `auth.generate_token` | Call |  | yes |  |
| `auth.login` | Call |  | yes |  |
| `auth.login_ex` | Call |  | yes |  |
| `auth.login_with_api_key` | Call |  | yes |  |
| `auth.login_with_token` | Call |  | yes |  |
| `auth.mechanism_choices` | Call |  | yes |  |
| `auth.twofactor.config` | CallRead |  | yes |  |
| `auth.twofactor.update` | Call |  | yes |  |
| `boot.environment.activate` | Call |  | yes |  |
| `boot.environment.clone` | Call |  | yes |  |
| `boot.environment.destroy` | Call |  | yes |  |
| `boot.environment.keep` | Call |  | yes |  |
| `boot.environment.query` | Call CallRead |  | yes |  |
| `catalog.config` | CallRead |  | yes |  |
| `catalog.trains` | CallRead |  | yes |  |
| `catalog.update` | Call |  | yes |  |
| `certificate.create` | CallJob |  | yes |  |
| `certificate.delete` | CallJob |  | yes |  |
| `certificate.get_instance` | CallRead |  | yes |  |
| `certificate.query` | Call CallRead |  | yes |  |
| `certificate.update` | CallJob |  | yes |  |
| `cloud_backup.create` | Call |  | yes |  |
| `cloud_backup.delete` | Call |  | yes |  |
| `cloud_backup.get_instance` | Call CallRead |  | yes |  |
| `cloud_backup.query` | Call CallRead |  | yes |  |
| `cloud_backup.update` | Call |  | yes |  |
| `cloudsync.create` | Call |  | yes |  |
| `cloudsync.credentials.create` | Call |  | yes |  |
| `cloudsync.credentials.delete` | Call |  | yes |  |
| `cloudsync.credentials.get_instance` | Call CallRead |  | yes |  |
| `cloudsync.credentials.query` | Call CallRead |  | yes |  |
| `cloudsync.credentials.update` | Call |  | yes |  |
| `cloudsync.delete` | Call |  | yes |  |
| `cloudsync.get_instance` | CallRead |  | yes |  |
| `cloudsync.query` | CallRead |  | yes |  |
| `cloudsync.update` | Call |  | yes |  |
| `container.create` | Call CallJob | 26.0 | yes |  |
| `container.delete` | Call | 26.0 | yes |  |
| `container.device.create` | Call | 26.0 | yes |  |
| `container.device.delete` | Call | 26.0 | yes |  |
| `container.device.get_instance` | Call CallRead | 26.0 | yes |  |
| `container.device.query` | Call | 26.0 | yes |  |
| `container.device.update` | Call | 26.0 | yes |  |
| `container.get_instance` | Call CallRead | 26.0 | yes |  |
| `container.image.query_registry` | CallRead | 26.0 | yes |  |
| `container.query` | Call CallRead | 26.0 | yes |  |
| `container.start` | Call | 26.0 | yes |  |
| `container.stop` | Call CallJob | 26.0 | yes |  |
| `container.update` | Call | 26.0 | yes |  |
| `core.get_jobs` | Call CallRead |  | yes |  |
| `core.get_methods` | Call |  | yes |  |
| `cronjob.create` | Call |  | yes |  |
| `cronjob.delete` | Call |  | yes |  |
| `cronjob.get_instance` | CallRead |  | yes |  |
| `cronjob.query` | Call CallRead |  | yes |  |
| `cronjob.update` | Call |  | yes |  |
| `directoryservices.config` | Call CallRead |  | yes |  |
| `directoryservices.status` | Call CallRead |  | yes |  |
| `directoryservices.update` | CallJob |  | yes |  |
| `docker.config` | CallRead |  | yes |  |
| `docker.network.query` | CallRead |  | yes |  |
| `docker.update` | CallJob |  | yes |  |
| `enclosure.label.set` | Call |  | yes |  |
| `enclosure2.query` | Call CallRead |  | yes |  |
| `failover.become_passive` | Call |  | yes |  |
| `failover.config` | Call CallRead |  | yes |  |
| `failover.disabled.reasons` | CallRead |  | yes |  |
| `failover.licensed` | CallRead |  | yes |  |
| `failover.node` | CallRead |  | yes |  |
| `failover.status` | CallRead |  | yes |  |
| `failover.update` | Call |  | yes |  |
| `filesystem.acltemplate.create` | Call |  | yes |  |
| `filesystem.acltemplate.delete` | Call |  | yes |  |
| `filesystem.acltemplate.get_instance` | Call CallRead |  | yes |  |
| `filesystem.acltemplate.query` | Call CallRead |  | yes |  |
| `filesystem.acltemplate.update` | Call |  | yes |  |
| `filesystem.getacl` | Call CallRead |  | yes |  |
| `filesystem.setacl` | CallJob |  | yes |  |
| `filesystem.setperm` | CallJob |  | yes |  |
| `filesystem.stat` | Call CallRead |  | yes |  |
| `ftp.config` | Call CallRead |  | yes |  |
| `ftp.update` | Call |  | yes |  |
| `group.create` | Call |  | yes |  |
| `group.delete` | Call |  | yes |  |
| `group.get_instance` | CallRead |  | yes |  |
| `group.query` | Call CallRead |  | yes |  |
| `group.update` | Call |  | yes |  |
| `initshutdownscript.create` | Call |  | yes |  |
| `initshutdownscript.delete` | Call |  | yes |  |
| `initshutdownscript.get_instance` | CallRead |  | yes |  |
| `initshutdownscript.query` | Call CallRead |  | yes |  |
| `initshutdownscript.update` | Call |  | yes |  |
| `interface.checkin` | Call |  | yes |  |
| `interface.commit` | Call |  | yes |  |
| `interface.create` | Call |  | yes |  |
| `interface.delete` | Call |  | yes |  |
| `interface.get_instance` | CallRead |  | yes |  |
| `interface.query` | Call CallRead |  | yes |  |
| `interface.rollback` | Call |  | yes |  |
| `interface.update` | Call |  | yes |  |
| `ipmi.lan.channels` | Call |  | yes |  |
| `ipmi.lan.query` | Call CallRead |  | yes |  |
| `ipmi.lan.update` | Call |  | yes |  |
| `iscsi.auth.create` | Call |  | yes |  |
| `iscsi.auth.delete` | Call |  | yes |  |
| `iscsi.auth.get_instance` | CallRead |  | yes |  |
| `iscsi.auth.query` | Call CallRead |  | yes |  |
| `iscsi.auth.update` | Call |  | yes |  |
| `iscsi.extent.create` | Call |  | yes |  |
| `iscsi.extent.delete` | Call |  | yes |  |
| `iscsi.extent.get_instance` | CallRead |  | yes |  |
| `iscsi.extent.query` | Call CallRead |  | yes |  |
| `iscsi.extent.update` | Call |  | yes |  |
| `iscsi.global.config` | Call CallRead |  | yes |  |
| `iscsi.global.update` | Call |  | yes |  |
| `iscsi.initiator.create` | Call |  | yes |  |
| `iscsi.initiator.delete` | CallJob |  | yes |  |
| `iscsi.initiator.get_instance` | CallRead |  | yes |  |
| `iscsi.initiator.query` | Call CallRead |  | yes |  |
| `iscsi.initiator.update` | Call |  | yes |  |
| `iscsi.portal.create` | Call |  | yes |  |
| `iscsi.portal.delete` | Call |  | yes |  |
| `iscsi.portal.get_instance` | CallRead |  | yes |  |
| `iscsi.portal.query` | Call CallRead |  | yes |  |
| `iscsi.portal.update` | Call |  | yes |  |
| `iscsi.target.create` | Call |  | yes |  |
| `iscsi.target.delete` | CallJob |  | yes |  |
| `iscsi.target.get_instance` | CallRead |  | yes |  |
| `iscsi.target.query` | Call CallRead |  | yes |  |
| `iscsi.target.update` | Call |  | yes |  |
| `iscsi.targetextent.create` | Call |  | yes |  |
| `iscsi.targetextent.delete` | Call |  | yes |  |
| `iscsi.targetextent.get_instance` | CallRead |  | yes |  |
| `iscsi.targetextent.update` | Call |  | yes |  |
| `kerberos.config` | Call CallRead |  | yes |  |
| `kerberos.keytab.create` | Call |  | yes |  |
| `kerberos.keytab.delete` | Call |  | yes |  |
| `kerberos.keytab.get_instance` | Call CallRead |  | yes |  |
| `kerberos.keytab.query` | Call CallRead |  | yes |  |
| `kerberos.keytab.update` | Call |  | yes |  |
| `kerberos.realm.create` | Call |  | yes |  |
| `kerberos.realm.delete` | Call |  | yes |  |
| `kerberos.realm.get_instance` | CallRead |  | yes |  |
| `kerberos.realm.query` | Call CallRead |  | yes |  |
| `kerberos.realm.update` | Call |  | yes |  |
| `kerberos.update` | Call |  | yes |  |
| `keychaincredential.create` | Call |  | yes |  |
| `keychaincredential.delete` | Call |  | yes |  |
| `keychaincredential.generate_ssh_key_pair` | Call |  | yes |  |
| `keychaincredential.get_instance` | CallRead |  | yes |  |
| `keychaincredential.query` | Call CallRead |  | yes |  |
| `keychaincredential.remote_ssh_host_key_scan` | Call |  |  | yes |
| `keychaincredential.update` | Call |  | yes |  |
| `lxc.config` | CallRead | 26.0 | yes |  |
| `lxc.update` | Call | 26.0 | yes |  |
| `mail.config` | Call CallRead |  | yes |  |
| `mail.update` | Call |  | yes |  |
| `network.configuration.config` | CallRead |  | yes |  |
| `network.configuration.update` | Call |  | yes |  |
| `nfs.config` | Call CallRead |  | yes |  |
| `nfs.update` | Call |  | yes |  |
| `nvmet.global.config` | Call CallRead |  | yes |  |
| `nvmet.global.update` | Call |  | yes |  |
| `nvmet.host.create` | Call |  | yes |  |
| `nvmet.host.delete` | Call |  | yes |  |
| `nvmet.host.get_instance` | CallRead |  | yes |  |
| `nvmet.host.query` | Call CallRead |  | yes |  |
| `nvmet.host.update` | Call |  | yes |  |
| `nvmet.host_subsys.create` | Call |  | yes |  |
| `nvmet.host_subsys.delete` | Call |  | yes |  |
| `nvmet.host_subsys.get_instance` | CallRead |  | yes |  |
| `nvmet.host_subsys.query` | Call |  |  | yes |
| `nvmet.namespace.create` | Call |  | yes |  |
| `nvmet.namespace.delete` | Call |  | yes |  |
| `nvmet.namespace.get_instance` | CallRead |  | yes |  |
| `nvmet.namespace.query` | Call CallRead |  | yes |  |
| `nvmet.namespace.update` | Call |  | yes |  |
| `nvmet.port.create` | Call |  | yes |  |
| `nvmet.port.delete` | Call |  | yes |  |
| `nvmet.port.get_instance` | CallRead |  | yes |  |
| `nvmet.port.query` | Call |  |  | yes |
| `nvmet.port.update` | Call |  | yes |  |
| `nvmet.port_subsys.create` | Call |  | yes |  |
| `nvmet.port_subsys.delete` | Call |  | yes |  |
| `nvmet.port_subsys.get_instance` | CallRead |  | yes |  |
| `nvmet.subsys.create` | Call |  | yes |  |
| `nvmet.subsys.delete` | Call |  | yes |  |
| `nvmet.subsys.get_instance` | CallRead |  | yes |  |
| `nvmet.subsys.query` | Call CallRead |  | yes |  |
| `nvmet.subsys.update` | Call |  | yes |  |
| `pool.create` | CallJob |  | yes |  |
| `pool.dataset.create` | Call |  | yes |  |
| `pool.dataset.delete` | Call CallJob |  | yes |  |
| `pool.dataset.get_instance` | Call CallRead |  | yes |  |
| `pool.dataset.query` | Call |  |  | yes |
| `pool.dataset.update` | Call |  | yes |  |
| `pool.delete` | CallJob |  | yes |  |
| `pool.get_instance` | CallRead |  | yes |  |
| `pool.query` | Call CallRead |  | yes |  |
| `pool.resilver.config` | Call CallRead |  | yes |  |
| `pool.resilver.update` | Call |  | yes |  |
| `pool.scrub.create` | Call |  | yes |  |
| `pool.scrub.delete` | Call |  | yes |  |
| `pool.scrub.get_instance` | CallRead |  | yes |  |
| `pool.scrub.query` | Call CallRead |  | yes |  |
| `pool.scrub.update` | Call |  | yes |  |
| `pool.snapshot.create` | Call |  | yes |  |
| `pool.snapshot.delete` | Call |  | yes |  |
| `pool.snapshot.get_instance` | CallRead |  | yes |  |
| `pool.snapshot.query` | Call |  |  | yes |
| `pool.snapshottask.create` | CallJob |  | yes |  |
| `pool.snapshottask.delete` | CallJob |  | yes |  |
| `pool.snapshottask.get_instance` | CallRead |  | yes |  |
| `pool.snapshottask.query` | Call CallRead |  | yes |  |
| `pool.snapshottask.update` | CallJob |  | yes |  |
| `pool.update` | CallJob |  | yes |  |
| `privilege.create` | Call |  | yes |  |
| `privilege.delete` | Call |  | yes |  |
| `privilege.get_instance` | CallRead |  | yes |  |
| `privilege.query` | Call CallRead |  | yes |  |
| `privilege.update` | Call |  | yes |  |
| `replication.config.config` | Call CallRead |  | yes |  |
| `replication.config.update` | Call |  | yes |  |
| `replication.create` | Call |  | yes |  |
| `replication.delete` | Call |  | yes |  |
| `replication.get_instance` | CallRead |  | yes |  |
| `replication.query` | Call CallRead |  | yes |  |
| `replication.run` | CallJob |  |  | yes |
| `replication.update` | Call |  | yes |  |
| `reporting.exporters.create` | Call |  | yes |  |
| `reporting.exporters.delete` | Call |  | yes |  |
| `reporting.exporters.get_instance` | CallRead |  | yes |  |
| `reporting.exporters.query` | Call CallRead |  | yes |  |
| `reporting.exporters.update` | Call |  | yes |  |
| `rsynctask.create` | Call |  | yes |  |
| `rsynctask.delete` | Call |  | yes |  |
| `rsynctask.get_instance` | CallRead |  | yes |  |
| `rsynctask.query` | Call CallRead |  | yes |  |
| `rsynctask.update` | Call |  | yes |  |
| `service.query` | Call CallRead |  | yes |  |
| `service.start` | CallJob |  | yes |  |
| `service.stop` | CallJob |  | yes |  |
| `service.update` | Call |  | yes |  |
| `sharing.nfs.create` | Call |  | yes |  |
| `sharing.nfs.delete` | Call |  | yes |  |
| `sharing.nfs.get_instance` | CallRead |  | yes |  |
| `sharing.nfs.query` | Call |  |  | yes |
| `sharing.nfs.update` | Call |  | yes |  |
| `sharing.smb.create` | Call |  | yes |  |
| `sharing.smb.delete` | Call CallJob |  | yes |  |
| `sharing.smb.get_instance` | Call CallRead |  | yes |  |
| `sharing.smb.query` | Call CallRead |  | yes |  |
| `sharing.smb.update` | Call |  | yes |  |
| `sharing.webshare.create` | Call | 26.0 | yes |  |
| `sharing.webshare.delete` | Call | 26.0 | yes |  |
| `sharing.webshare.get_instance` | Call CallRead | 26.0 | yes |  |
| `sharing.webshare.query` | Call CallRead | 26.0 | yes |  |
| `sharing.webshare.update` | Call | 26.0 | yes |  |
| `smb.config` | CallRead |  | yes |  |
| `smb.update` | Call |  | yes |  |
| `snmp.config` | Call CallRead |  | yes |  |
| `snmp.update` | Call |  | yes |  |
| `ssh.config` | CallRead |  | yes |  |
| `ssh.update` | Call |  | yes |  |
| `staticroute.create` | Call |  | yes |  |
| `staticroute.delete` | Call |  | yes |  |
| `staticroute.get_instance` | CallRead |  | yes |  |
| `staticroute.query` | Call CallRead |  | yes |  |
| `staticroute.update` | Call |  | yes |  |
| `system.advanced.config` | CallRead |  | yes |  |
| `system.advanced.update` | Call |  | yes |  |
| `system.general.config` | CallRead |  | yes |  |
| `system.general.update` | Call |  | yes |  |
| `system.ntpserver.create` | Call |  | yes |  |
| `system.ntpserver.delete` | Call |  | yes |  |
| `system.ntpserver.get_instance` | CallRead |  | yes |  |
| `system.ntpserver.query` | Call CallRead |  | yes |  |
| `system.ntpserver.update` | Call |  | yes |  |
| `system.version_short` | Call CallRead |  | yes |  |
| `systemdataset.config` | CallRead |  | yes |  |
| `systemdataset.update` | CallJob |  | yes |  |
| `tn_connect.config` | CallRead |  | yes |  |
| `tn_connect.update` | Call |  | yes |  |
| `truecommand.config` | CallRead |  | yes |  |
| `truecommand.update` | Call |  | yes |  |
| `tunable.create` | CallJob |  | yes |  |
| `tunable.delete` | CallJob |  | yes |  |
| `tunable.get_instance` | CallRead |  | yes |  |
| `tunable.query` | Call CallRead |  | yes |  |
| `tunable.update` | CallJob |  | yes |  |
| `ups.config` | Call CallRead |  | yes |  |
| `ups.update` | Call |  | yes |  |
| `user.create` | Call |  | yes |  |
| `user.delete` | Call |  | yes |  |
| `user.get_instance` | Call CallRead |  | yes |  |
| `user.get_next_uid` | CallJob |  |  | yes |
| `user.query` | Call CallRead |  | yes |  |
| `user.update` | Call |  | yes |  |
| `vm.create` | Call |  | yes |  |
| `vm.delete` | Call |  | yes |  |
| `vm.device.create` | Call |  | yes |  |
| `vm.device.delete` | Call |  | yes |  |
| `vm.device.get_instance` | CallRead |  | yes |  |
| `vm.device.query` | Call |  |  | yes |
| `vm.device.update` | Call |  | yes |  |
| `vm.get_instance` | CallRead |  | yes |  |
| `vm.query` | Call CallRead |  | yes |  |
| `vm.start` | Call |  | yes |  |
| `vm.stop` | CallJob |  | yes |  |
| `vm.update` | Call |  | yes |  |
| `vmware.create` | Call |  | yes |  |
| `vmware.delete` | Call |  | yes |  |
| `vmware.get_instance` | Call CallRead |  | yes |  |
| `vmware.update` | Call |  | yes |  |
| `webshare.config` | CallRead | 26.0 | yes |  |
| `webshare.update` | Call | 26.0 | yes |  |

## Namespaces

- **`acme`** (5): `acme.dns.authenticator.create`, `acme.dns.authenticator.delete`, `acme.dns.authenticator.get_instance`, `acme.dns.authenticator.query`, `acme.dns.authenticator.update`
- **`alertclasses`** (2): `alertclasses.config`, `alertclasses.update`
- **`alertservice`** (5): `alertservice.create`, `alertservice.delete`, `alertservice.get_instance`, `alertservice.query`, `alertservice.update`
- **`api_key`** (5): `api_key.create`, `api_key.delete`, `api_key.get_instance`, `api_key.query`, `api_key.update`
- **`app`** (13): `app.create`, `app.delete`, `app.get_instance`, `app.query`, `app.registry.create`, `app.registry.delete`, `app.registry.get_instance`, `app.registry.query`, `app.registry.update`, `app.start`, `app.stop`, `app.update`, `app.upgrade`
- **`audit`** (2): `audit.config`, `audit.update`
- **`auth`** (8): `auth.generate_token`, `auth.login`, `auth.login_ex`, `auth.login_with_api_key`, `auth.login_with_token`, `auth.mechanism_choices`, `auth.twofactor.config`, `auth.twofactor.update`
- **`boot`** (5): `boot.environment.activate`, `boot.environment.clone`, `boot.environment.destroy`, `boot.environment.keep`, `boot.environment.query`
- **`catalog`** (3): `catalog.config`, `catalog.trains`, `catalog.update`
- **`certificate`** (5): `certificate.create`, `certificate.delete`, `certificate.get_instance`, `certificate.query`, `certificate.update`
- **`cloud_backup`** (5): `cloud_backup.create`, `cloud_backup.delete`, `cloud_backup.get_instance`, `cloud_backup.query`, `cloud_backup.update`
- **`cloudsync`** (10): `cloudsync.create`, `cloudsync.credentials.create`, `cloudsync.credentials.delete`, `cloudsync.credentials.get_instance`, `cloudsync.credentials.query`, `cloudsync.credentials.update`, `cloudsync.delete`, `cloudsync.get_instance`, `cloudsync.query`, `cloudsync.update`
- **`container`** (13): `container.create`, `container.delete`, `container.device.create`, `container.device.delete`, `container.device.get_instance`, `container.device.query`, `container.device.update`, `container.get_instance`, `container.image.query_registry`, `container.query`, `container.start`, `container.stop`, `container.update`
- **`core`** (2): `core.get_jobs`, `core.get_methods`
- **`cronjob`** (5): `cronjob.create`, `cronjob.delete`, `cronjob.get_instance`, `cronjob.query`, `cronjob.update`
- **`directoryservices`** (3): `directoryservices.config`, `directoryservices.status`, `directoryservices.update`
- **`docker`** (3): `docker.config`, `docker.network.query`, `docker.update`
- **`enclosure`** (1): `enclosure.label.set`
- **`enclosure2`** (1): `enclosure2.query`
- **`failover`** (7): `failover.become_passive`, `failover.config`, `failover.disabled.reasons`, `failover.licensed`, `failover.node`, `failover.status`, `failover.update`
- **`filesystem`** (9): `filesystem.acltemplate.create`, `filesystem.acltemplate.delete`, `filesystem.acltemplate.get_instance`, `filesystem.acltemplate.query`, `filesystem.acltemplate.update`, `filesystem.getacl`, `filesystem.setacl`, `filesystem.setperm`, `filesystem.stat`
- **`ftp`** (2): `ftp.config`, `ftp.update`
- **`group`** (5): `group.create`, `group.delete`, `group.get_instance`, `group.query`, `group.update`
- **`initshutdownscript`** (5): `initshutdownscript.create`, `initshutdownscript.delete`, `initshutdownscript.get_instance`, `initshutdownscript.query`, `initshutdownscript.update`
- **`interface`** (8): `interface.checkin`, `interface.commit`, `interface.create`, `interface.delete`, `interface.get_instance`, `interface.query`, `interface.rollback`, `interface.update`
- **`ipmi`** (3): `ipmi.lan.channels`, `ipmi.lan.query`, `ipmi.lan.update`
- **`iscsi`** (31): `iscsi.auth.create`, `iscsi.auth.delete`, `iscsi.auth.get_instance`, `iscsi.auth.query`, `iscsi.auth.update`, `iscsi.extent.create`, `iscsi.extent.delete`, `iscsi.extent.get_instance`, `iscsi.extent.query`, `iscsi.extent.update`, `iscsi.global.config`, `iscsi.global.update`, `iscsi.initiator.create`, `iscsi.initiator.delete`, `iscsi.initiator.get_instance`, `iscsi.initiator.query`, `iscsi.initiator.update`, `iscsi.portal.create`, `iscsi.portal.delete`, `iscsi.portal.get_instance`, `iscsi.portal.query`, `iscsi.portal.update`, `iscsi.target.create`, `iscsi.target.delete`, `iscsi.target.get_instance`, `iscsi.target.query`, `iscsi.target.update`, `iscsi.targetextent.create`, `iscsi.targetextent.delete`, `iscsi.targetextent.get_instance`, `iscsi.targetextent.update`
- **`kerberos`** (12): `kerberos.config`, `kerberos.keytab.create`, `kerberos.keytab.delete`, `kerberos.keytab.get_instance`, `kerberos.keytab.query`, `kerberos.keytab.update`, `kerberos.realm.create`, `kerberos.realm.delete`, `kerberos.realm.get_instance`, `kerberos.realm.query`, `kerberos.realm.update`, `kerberos.update`
- **`keychaincredential`** (7): `keychaincredential.create`, `keychaincredential.delete`, `keychaincredential.generate_ssh_key_pair`, `keychaincredential.get_instance`, `keychaincredential.query`, `keychaincredential.remote_ssh_host_key_scan`, `keychaincredential.update`
- **`lxc`** (2): `lxc.config`, `lxc.update`
- **`mail`** (2): `mail.config`, `mail.update`
- **`network`** (2): `network.configuration.config`, `network.configuration.update`
- **`nfs`** (2): `nfs.config`, `nfs.update`
- **`nvmet`** (29): `nvmet.global.config`, `nvmet.global.update`, `nvmet.host.create`, `nvmet.host.delete`, `nvmet.host.get_instance`, `nvmet.host.query`, `nvmet.host.update`, `nvmet.host_subsys.create`, `nvmet.host_subsys.delete`, `nvmet.host_subsys.get_instance`, `nvmet.host_subsys.query`, `nvmet.namespace.create`, `nvmet.namespace.delete`, `nvmet.namespace.get_instance`, `nvmet.namespace.query`, `nvmet.namespace.update`, `nvmet.port.create`, `nvmet.port.delete`, `nvmet.port.get_instance`, `nvmet.port.query`, `nvmet.port.update`, `nvmet.port_subsys.create`, `nvmet.port_subsys.delete`, `nvmet.port_subsys.get_instance`, `nvmet.subsys.create`, `nvmet.subsys.delete`, `nvmet.subsys.get_instance`, `nvmet.subsys.query`, `nvmet.subsys.update`
- **`pool`** (26): `pool.create`, `pool.dataset.create`, `pool.dataset.delete`, `pool.dataset.get_instance`, `pool.dataset.query`, `pool.dataset.update`, `pool.delete`, `pool.get_instance`, `pool.query`, `pool.resilver.config`, `pool.resilver.update`, `pool.scrub.create`, `pool.scrub.delete`, `pool.scrub.get_instance`, `pool.scrub.query`, `pool.scrub.update`, `pool.snapshot.create`, `pool.snapshot.delete`, `pool.snapshot.get_instance`, `pool.snapshot.query`, `pool.snapshottask.create`, `pool.snapshottask.delete`, `pool.snapshottask.get_instance`, `pool.snapshottask.query`, `pool.snapshottask.update`, `pool.update`
- **`privilege`** (5): `privilege.create`, `privilege.delete`, `privilege.get_instance`, `privilege.query`, `privilege.update`
- **`replication`** (8): `replication.config.config`, `replication.config.update`, `replication.create`, `replication.delete`, `replication.get_instance`, `replication.query`, `replication.run`, `replication.update`
- **`reporting`** (5): `reporting.exporters.create`, `reporting.exporters.delete`, `reporting.exporters.get_instance`, `reporting.exporters.query`, `reporting.exporters.update`
- **`rsynctask`** (5): `rsynctask.create`, `rsynctask.delete`, `rsynctask.get_instance`, `rsynctask.query`, `rsynctask.update`
- **`service`** (4): `service.query`, `service.start`, `service.stop`, `service.update`
- **`sharing`** (15): `sharing.nfs.create`, `sharing.nfs.delete`, `sharing.nfs.get_instance`, `sharing.nfs.query`, `sharing.nfs.update`, `sharing.smb.create`, `sharing.smb.delete`, `sharing.smb.get_instance`, `sharing.smb.query`, `sharing.smb.update`, `sharing.webshare.create`, `sharing.webshare.delete`, `sharing.webshare.get_instance`, `sharing.webshare.query`, `sharing.webshare.update`
- **`smb`** (2): `smb.config`, `smb.update`
- **`snmp`** (2): `snmp.config`, `snmp.update`
- **`ssh`** (2): `ssh.config`, `ssh.update`
- **`staticroute`** (5): `staticroute.create`, `staticroute.delete`, `staticroute.get_instance`, `staticroute.query`, `staticroute.update`
- **`system`** (10): `system.advanced.config`, `system.advanced.update`, `system.general.config`, `system.general.update`, `system.ntpserver.create`, `system.ntpserver.delete`, `system.ntpserver.get_instance`, `system.ntpserver.query`, `system.ntpserver.update`, `system.version_short`
- **`systemdataset`** (2): `systemdataset.config`, `systemdataset.update`
- **`tn_connect`** (2): `tn_connect.config`, `tn_connect.update`
- **`truecommand`** (2): `truecommand.config`, `truecommand.update`
- **`tunable`** (5): `tunable.create`, `tunable.delete`, `tunable.get_instance`, `tunable.query`, `tunable.update`
- **`ups`** (2): `ups.config`, `ups.update`
- **`user`** (6): `user.create`, `user.delete`, `user.get_instance`, `user.get_next_uid`, `user.query`, `user.update`
- **`vm`** (12): `vm.create`, `vm.delete`, `vm.device.create`, `vm.device.delete`, `vm.device.get_instance`, `vm.device.query`, `vm.device.update`, `vm.get_instance`, `vm.query`, `vm.start`, `vm.stop`, `vm.update`
- **`vmware`** (4): `vmware.create`, `vmware.delete`, `vmware.get_instance`, `vmware.update`
- **`webshare`** (2): `webshare.config`, `webshare.update`

## Method-shaped literals not at a direct call site

Namespaced-method-looking strings referenced indirectly (constants, maps, subscription channels, probes). Review — some are real methods invoked via a wrapper, some are false positives (e.g. attribute keys).

- `app.registry` — `internal/resources/app_registry/schema.go`
- `auth.me` — `cmd/debug_api/main.go`
- `container.device.gpu_choices` — `cmd/debug_api/main.go`
- `container.device.nic_attach_choices` — `cmd/debug_api/main.go`
- `container.device.usb_choices` — `cmd/debug_api/main.go`
- `container.pool_choices` — `cmd/debug_api/main.go`
- `enclosure.get_instance` — `cmd/debug_api/main.go`
- `enclosure.query` — `cmd/debug_api/main.go`
- `failover.disabled_reasons` — `cmd/debug_api/main.go`
- `filesystem.put` — `internal/acctest/acctest.go`
- `ipmi.is_loaded` — `cmd/debug_api/main.go`
- `iscsi.targetextent.query` — `cmd/debug_api/main.go`
- `lxc.bridge_choices` — `cmd/debug_api/main.go`
- `nvmet.port_subsys.query` — `cmd/debug_api/main.go`
- `vmware.query` — `cmd/debug_api/main.go`
- `webshare.bindip_choices` — `cmd/debug_api/main.go`

