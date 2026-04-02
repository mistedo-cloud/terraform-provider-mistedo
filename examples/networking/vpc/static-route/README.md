# Compute — VPC static route (`mistedo_vpc_route`)

**Required-only** resource: **`destination`** + **`nexthop`**. The provider resolves **`router_id`** from `GET .../network_routers`.

Set **`peer_cidr`** / **`peer_nexthop`** to a real pair your OpenStack/router accepts. Wrong values may fail at apply or break traffic.

Import: `terraform import mistedo_vpc_route.to_peer '10.220.0.0/16|10.220.0.2'`
