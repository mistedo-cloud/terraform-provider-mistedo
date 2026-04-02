# Scenario: same as private-network-minimal, plus optional dns_nameservers (tracked in state).

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
  name              = var.network_name
  cidr              = var.network_cidr
  dns_nameservers   = var.dns_nameservers
}

output "network_id" {
  value = mistedo_network.app.id
}
