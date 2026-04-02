---
page_title: "mistedo_lb_backend_services Data Source - Mistedo Terraform Provider"
subcategory: "ALB"
description: |-
  Lists backend services (instance groups) that can be attached to mistedo_lb_route.
---

# mistedo_lb_backend_services (Data Source)

Lists **backend services** — compute targets that serve HTTP behind the load balancer.

Each **`id`** is used as **`service_id`** inside the `services` block of [`mistedo_lb_route`](../resources/lb_route.md).

## Example

```hcl
data "mistedo_load_balancers" "all" {}

data "mistedo_lb_backend_services" "all" {}

resource "mistedo_lb_route" "app" {
  name             = "app"
  hostname         = "app.example.com"
  cloud_gateway_id = data.mistedo_load_balancers.all.gateways[0].id

  services = [
    {
      service_id = data.mistedo_lb_backend_services.all.services[0].id
    },
  ]
}
```

See also [`mistedo_lb_route`](../resources/lb_route.md) for full route examples.

## Arguments

This data source has **no** configuration arguments.

## Attributes

| Name | Description |
|------|-------------|
| `services` | List of service objects (see below). |

### `services` objects

| Field | Description |
|-------|-------------|
| `id` | **Service id** — required for `mistedo_lb_route.services`. |
| `ext_id` | External compute identifier. |
| `name` | Display name. |
| `ipaddresses` | Comma-separated backend IPs (string from API). |
| `owner` | Owner from API. |
| `account` | Account identifier. |

## Notes

- Prefer stable selection (by **`name`** or **`id`**) instead of hard-coding `[0]`.
