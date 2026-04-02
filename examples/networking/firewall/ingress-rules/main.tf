# Scenario: app firewall group with typical ingress rules (required + optional rule fields).
# - direction (required) + security_group_id (required)
# - protocol, port_range, network_protocol, remote_ip_subnet (optional but used here for clarity)

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

resource "mistedo_security_group" "app" {
  name = "tf_example_app_fw"
}

resource "mistedo_security_group_rule" "ssh" {
  security_group_id = mistedo_security_group.app.id
  direction         = "ingress"
  protocol          = "tcp"
  port_range        = "22"
  network_protocol  = "IPV4"
  remote_ip_subnet  = var.admin_cidr
}

resource "mistedo_security_group_rule" "https" {
  security_group_id = mistedo_security_group.app.id
  direction         = "ingress"
  protocol          = "tcp"
  port_range        = "443"
  network_protocol  = "IPV4"
  remote_ip_subnet  = var.admin_cidr
}

output "security_group_id" {
  value = mistedo_security_group.app.id
}
