---
page_title: "mistedo_load_balancers Data Source - terraform-provider-mistedo"
subcategory: "ALB"
description: |-
  Lists load balancer gateways for the account; use gateway id when creating mistedo_lb_route.
---

# mistedo_load_balancers (Data Source)

**Read-only.** Fetches the list of **Cloud Gateways** (application load balancers) available to your account.

Use a gateway’s **`id`** as **`cloud_gateway_id`** on [`mistedo_lb_route`](../resources/lb_route.md).

---

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

In real configs, choose the gateway that matches your environment (by `name` or `id`), not always `[0]`.

---

## Attributes

`gateways` — list of objects:

| Field | Meaning |
|-------|--------|
| `id` | **Numeric id** — use on routes. |
| `name` | Human-readable name (e.g. `alb-default`). |
| `cloudgw_id` | CloudGateway VM UUID. |
| `cloudgw_instance` | Instance display name. |
| `account` | Account identifier. |
