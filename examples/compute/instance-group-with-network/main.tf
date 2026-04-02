# Scenario: private network + firewall + catalog template lookup + instance group order.
# - mistedo_instance_group.subnet must be the API subnet name (use mistedo_network.canonical_name).
# - boot_disk is a SingleNestedAttribute: use boot_disk = { ... }, not a boot_disk { } block.
# - Omit password to auto-generate; do not set password = "" (Terraform sensitive plan rules).

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

resource "mistedo_security_group" "app" {
  name = var.security_group_name
}

resource "mistedo_security_group_rule" "ssh" {
  security_group_id = mistedo_security_group.app.id
  direction         = "ingress"
  protocol          = "tcp"
  port_range        = "22"
  network_protocol  = "IPV4"
  remote_ip_subnet  = var.ssh_ingress_cidr
}

data "mistedo_instance_template" "os" {
  name    = var.template_name
  version = var.template_version != "" ? var.template_version : null
}

resource "mistedo_instance_group" "app" {
  name        = var.instance_group_name
  description = "Example instance group from examples/compute/instance-group-with-network"

  template_id            = data.mistedo_instance_template.os.id
  desired_instance_count = 1

  cpu_cores  = 1
  memory_mib = 1024

  boot_disk = {
    size_gib = 10
    type     = "nvme"
  }

  subnet                = mistedo_network.app.canonical_name
  private_ip_allocation = "auto"

  security_group_ids = [mistedo_security_group.app.id]

  pass_auth = "disable_password"

  public_remote_access = ["22/tcp"]
  managed_access       = "enable"
}

output "network_id" {
  value = mistedo_network.app.id
}

output "network_canonical_name" {
  description = "Subnet name used in the instance group order (API form)."
  value       = mistedo_network.app.canonical_name
}

output "security_group_id" {
  value = mistedo_security_group.app.id
}

output "instance_template_id" {
  value = data.mistedo_instance_template.os.id
}

output "instance_group_id" {
  value = mistedo_instance_group.app.id
}

output "instance_group_private_ipv4_cidr" {
  value = mistedo_instance_group.app.private_ipv4_cidr
}

output "instance_group_instances" {
  value = mistedo_instance_group.app.instances
}
