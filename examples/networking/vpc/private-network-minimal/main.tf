# Scenario: private network with only required arguments (name + cidr).
# ip_version / network_protocol are fixed inside the provider; dns_nameservers omitted.

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

resource "mistedo_network" "app" {
  name = var.network_name
  cidr = var.network_cidr
}

output "network_id" {
  value = mistedo_network.app.id
}

output "subnet_id" {
  value = mistedo_network.app.subnet_id
}

output "canonical_name" {
  value = mistedo_network.app.canonical_name
}
