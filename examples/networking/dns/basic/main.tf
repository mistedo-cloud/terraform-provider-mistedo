# Scenario: create a zone and a single A record using only schema-required fields.
# Optional record fields (ttl, priority, …) are omitted so provider defaults apply where defined.

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

# Required only: zone, name, type, content (ttl defaults to 300 in the provider).
resource "mistedo_dns_record" "www" {
  zone    = mistedo_dns_zone.site.name
  name    = "www"
  type    = "A"
  content = var.www_content
}

output "zone_id" {
  value = mistedo_dns_zone.site.id
}

output "record_id" {
  value = mistedo_dns_record.www.id
}
