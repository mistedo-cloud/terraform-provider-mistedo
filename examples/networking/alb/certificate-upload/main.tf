# Scenario: upload TLS material with only required attributes (name + PEMs).
# Optional: ca_pem, dest_ca_pem, owner — omitted here.

provider "mistedo" {
  username = var.mistedo_username
  password = var.mistedo_password
  account  = var.mistedo_account
  location = var.mistedo_location
  role     = var.mistedo_role

  auth_url    = "https://auth.dev.mistedo.by/realms"
  auth_realm  = "master"
  auth_client = "cloud-console"
}

resource "tls_private_key" "example" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_self_signed_cert" "example" {
  private_key_pem = tls_private_key.example.private_key_pem
  subject {
    common_name = var.cert_name
  }
  validity_period_hours = 720
  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]
  dns_names = [var.cert_name]
}

resource "mistedo_lb_certificate" "example" {
  name            = var.cert_name
  certificate_pem = tls_self_signed_cert.example.cert_pem
  private_key_pem = tls_private_key.example.private_key_pem
}

output "certificate_id" {
  value       = mistedo_lb_certificate.example.id
  description = "Pass to mistedo_lb_route.certificate_id as tonumber(...)."
}
