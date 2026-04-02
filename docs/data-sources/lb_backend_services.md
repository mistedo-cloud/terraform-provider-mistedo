---
page_title: "mistedo_lb_backend_services Data Source - terraform-provider-mistedo"
subcategory: "ALB"
description: |-
  Lists backend services (instance groups) that can be attached to mistedo_lb_route.
---

# mistedo_lb_backend_services (Data Source)

**Read-only.** Lists **backend services** — the compute/instance targets that actually serve HTTP behind the load balancer.

Each service has an **`id`** you pass as **`service_id`** inside the `services` block of [`mistedo_lb_route`](../resources/lb_route.md).

---

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

Combine with [`mistedo_load_balancers`](load_balancers.md) as in the [lb_route examples](../resources/lb_route.md).

---

## Attributes

`services` — list of objects:

| Field | Meaning |
|-------|--------|
| `id` | **Internal service id** — required for routes. |
| `ext_id` | External compute identifier. |
| `name` | Display name. |
| `ipaddresses` | Comma-separated backend IPs (string from API). |
| `owner` | Owner from API. |
| `account` | Account identifier. |
