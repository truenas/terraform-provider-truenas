# Security Policy

## Reporting a vulnerability

Please report suspected security vulnerabilities privately, not through a
public GitHub issue.

- Email: **security@truenas.com**
- Or use GitHub's private vulnerability reporting on this repository
  (Security → Report a vulnerability).

Include the affected provider version, the resource(s) or code path
involved, a description of the issue, and reproduction steps if you have
them. You will receive an acknowledgement, and we will keep you informed as
the report is investigated and resolved.

Please give us a reasonable period to release a fix before any public
disclosure.

## Scope

This policy covers the Terraform provider in this repository — the code that
runs on the operator's machine and connects to TrueNAS. Vulnerabilities
in TrueNAS itself (the middleware/server) should be reported through the
TrueNAS security process; where such an issue affects
how this provider should behave, note that in your report.

## Handling of secrets

The provider treats credentials as follows, and reports that deviate from
this are in scope:

- Write-only secrets (passwords, CHAP/DH-CHAP keys, bind passwords, registry
  and cloud credentials, SED passwords) are sourced from configuration and
  never written to Terraform state.
- Secrets the API returns intact (certificate private keys, Kerberos
  keytabs, SSH private keys) are marked `Sensitive` and masked in plan
  output.
- No secret value is written to logs or interpolated into error messages.
- Disabling TLS verification (`insecure = true`) also weakens
  authentication — see GETTING-STARTED.md. Use `ca_cert` for untrusted
  networks.
