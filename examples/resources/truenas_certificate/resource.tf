# Generate a new key pair and CSR on the TrueNAS box itself
# (CERTIFICATE_CREATE_CSR) -- no key material needs to leave Terraform.
resource "truenas_certificate" "internal_csr" {
  name        = "internal-web-csr"
  create_type = "CERTIFICATE_CREATE_CSR"
  key_type    = "RSA"
  key_length  = 2048
  common      = "truenas.example.internal"
  san         = ["truenas.example.internal"]
}

# Import an existing certificate + private key pair
# (CERTIFICATE_CREATE_IMPORTED). Secrets are sourced from variables, never
# committed as literals.
resource "truenas_certificate" "imported" {
  name        = "imported-web-cert"
  create_type = "CERTIFICATE_CREATE_IMPORTED"
  certificate = var.certificate_pem
  privatekey  = var.certificate_privatekey_pem
}

variable "certificate_pem" {
  type = string
}

variable "certificate_privatekey_pem" {
  type      = string
  sensitive = true
}

# Import a certificate + key from an ephemeral source (Terraform >= 1.11) with the
# write-only attributes (#65): neither value is stored in state or saved plans.
# Rotation = bump certificate_wo_version; TrueNAS cannot replace the key under an
# existing certificate, so the bump REPLACES the resource (new id). With
# create_before_destroy the new certificate exists and every resource that
# references its id is re-pointed before the old one is destroyed. TrueNAS
# requires unique certificate names, so the name must change with the version.
ephemeral "vault_kv_secret_v2" "web_tls" {
  mount = "secret"
  name  = "web-tls"
}

variable "web_cert_version" {
  type    = number
  default = 1
}

resource "truenas_certificate" "imported_wo" {
  name                   = "imported-web-cert-v${var.web_cert_version}"
  create_type            = "CERTIFICATE_CREATE_IMPORTED"
  certificate_wo         = ephemeral.vault_kv_secret_v2.web_tls.data["tls_crt"]
  privatekey_wo          = ephemeral.vault_kv_secret_v2.web_tls.data["tls_key"]
  certificate_wo_version = var.web_cert_version

  lifecycle {
    create_before_destroy = true
  }
}
