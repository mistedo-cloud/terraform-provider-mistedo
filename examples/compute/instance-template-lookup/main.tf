# Scenario: look up one catalog VM template (service template) by name and optional version.
# Use the outputs as template_id when ordering mistedo_instance_group or other services.

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

data "mistedo_instance_template" "os" {
  name    = var.template_name
  version = var.template_version != "" ? var.template_version : null
}

output "template_id" {
  description = "ManageIQ service_template id for orders (e.g. mistedo_instance_group.template_id)."
  value       = data.mistedo_instance_template.os.id
}

output "template_full_name" {
  value = data.mistedo_instance_template.os.full_name
}

output "template_guid" {
  value = data.mistedo_instance_template.os.guid
}
