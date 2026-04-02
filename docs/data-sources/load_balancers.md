---
page_title: "mistedo_load_balancers Data Source - Mistedo Terraform Provider"
subcategory: "ALB"
description: |-
  Lists load balancer gateways for the account; use gateway id when creating mistedo_lb_route.
---

# mistedo_load_balancers (Data Source)

Lists **Cloud Gateways** (application load balancers) available to your account.

Use a gateway **`id`** as **`cloud_gateway_id`** on [`mistedo_lb_route`](../resources/lb_route.md).

## Example

```hcl
data "mistedo_load_balancers" "all" {}

output "first_gateway_id" {
  value = data.mistedo_load_balancers.all.gateways[0].id
}

output "first_gateway_name" {
  value = data.mistedo_load_balancers.all.gateways[0].name
}
```

In real configs, select the gateway by **`name`** or **`id`**, not only `[0]`.

## Arguments

This data source has **no** configuration arguments.

## Attributes

| Name | Description |
|------|-------------|
| `gateways` | List of gateway objects (see below). |

### `gateways` objects

| Field | Description |
|-------|-------------|
| `id` | **Numeric id** — use on [`mistedo_lb_route`](../resources/lb_route.md). |
| `name` | Human-readable name (e.g. `alb-default`). |
| `cloudgw_id` | CloudGateway VM UUID. |
| `cloudgw_instance` | Instance display name. |
| `account` | Account identifier. |

## Notes

- Combine with [`mistedo_lb_backend_services`](lb_backend_services.md) when building routes; see [`mistedo_lb_route`](../resources/lb_route.md) examples.
