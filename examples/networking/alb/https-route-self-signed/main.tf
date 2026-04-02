# Scenario: upload a self-signed cert (tls provider) + HTTPS route with optional healthcheck.
# If apply fails on TLS enums, set tls_termination / insecure to values your Traefik Manager expects.

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
    common_name = var.hostname
  }

  validity_period_hours = 2160 # 90d lab cert

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]

  dns_names = [var.hostname]
}

# Required: name, certificate_pem, private_key_pem. Optional: ca_pem, dest_ca_pem, owner.
resource "mistedo_lb_certificate" "example" {
  name            = var.hostname
  certificate_pem = tls_self_signed_cert.example.cert_pem
  private_key_pem = tls_private_key.example.private_key_pem
}

# certificate_id is optional; healthcheck is optional (nested fields optional inside the block).
resource "mistedo_lb_route" "app" {
  name             = var.route_name
  hostname         = var.hostname
  cloud_gateway_id = var.cloud_gateway_id
  certificate_id   = tonumber(mistedo_lb_certificate.example.id)
  target_port      = 443

  services = [
    {
      service_id = var.backend_service_id
    },
  ]

  healthcheck = {
    path     = var.healthcheck_path
    scheme   = "https"
    method   = "GET"
    interval = 15
    timeout  = 3
  }
}

output "certificate_id" {
  value = mistedo_lb_certificate.example.id
}

output "route_id" {
  value = mistedo_lb_route.app.id
}
