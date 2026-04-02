# Scenario: empty firewall group (required attribute: name only).
# After create the provider clears default egress rules so you can add mistedo_security_group_rule resources later.

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

resource "mistedo_security_group" "empty" {
  name = var.group_name
}

output "security_group_id" {
  value = mistedo_security_group.empty.id
}

output "canonical_name" {
  value = mistedo_security_group.empty.canonical_name
}
