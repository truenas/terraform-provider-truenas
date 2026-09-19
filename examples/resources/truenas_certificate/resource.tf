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
