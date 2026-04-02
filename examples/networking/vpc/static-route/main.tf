# Scenario: static route on the tenant default network router (no router_id in config).
# Replace peer_cidr / peer_nexthop with values that exist in your environment.

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

resource "mistedo_vpc_route" "to_peer" {
  destination = var.peer_cidr
  nexthop     = var.peer_nexthop
}

output "resolved_router_id" {
  value = mistedo_vpc_route.to_peer.router_id
}
