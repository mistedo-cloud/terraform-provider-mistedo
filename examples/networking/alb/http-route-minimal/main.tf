# Scenario: HTTP route with required fields only for the route + one backend.
# Optional attributes (path, target_port, ip_version, healthcheck, …) use provider/API defaults.

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

resource "mistedo_lb_route" "app" {
  name             = var.route_name
  hostname         = var.hostname
  cloud_gateway_id = var.cloud_gateway_id

  services = [
    {
      service_id = var.backend_service_id
    },
  ]
}

output "route_id" {
  value = mistedo_lb_route.app.id
}
