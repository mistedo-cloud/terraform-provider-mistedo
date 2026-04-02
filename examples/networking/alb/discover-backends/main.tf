# Scenario: read-only — list ALB gateways and backend services (no mistedo_lb_* resources).
# Use outputs to pick cloud_gateway_id and service_id for other examples.

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

data "mistedo_load_balancers" "all" {}

data "mistedo_lb_backend_services" "all" {}

output "gateways" {
  description = "Use id as mistedo_lb_route.cloud_gateway_id."
  value       = data.mistedo_load_balancers.all.gateways
}

output "services" {
  description = "Use id in mistedo_lb_route.services[].service_id."
  value       = data.mistedo_lb_backend_services.all.services
}
