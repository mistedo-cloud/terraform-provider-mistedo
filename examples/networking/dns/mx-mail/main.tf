# Scenario: hosted zone + MX record with optional priority (typical mail setup).

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

resource "mistedo_dns_zone" "site" {
  name = var.zone_name
}

# MX: optional priority is set explicitly; ttl still omitted (default 300).
resource "mistedo_dns_record" "mx" {
  zone     = mistedo_dns_zone.site.name
  name     = "@"
  type     = "MX"
  content  = var.mail_host
  priority = 10
}

output "zone_name" {
  value = mistedo_dns_zone.site.name
}
