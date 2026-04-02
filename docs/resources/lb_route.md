---
page_title: "mistedo_lb_route Resource - terraform-provider-mistedo"
subcategory: "ALB"
description: |-
  HTTP(S) route on the Mistedo application load balancer: hostname, backends, optional health check and TLS.
---

# mistedo_lb_route (Resource)

In plain terms: when a **visitor** opens a **hostname** (and path) on the platform **load balancer**, this resource defines **which backend services** receive the traffic and how **health checks** run.

You need:

1. A **gateway** id from [`mistedo_load_balancers`](../data-sources/load_balancers.md) → `cloud_gateway_id`.
2. One or more **backend service** ids from [`mistedo_lb_backend_services`](../data-sources/lb_backend_services.md) → `services`.

Updates send a **full** JSON body to the API; do not rely on merging partial changes outside Terraform.

---

## Example: simple route with health check

```hcl
data "mistedo_load_balancers" "all" {}

data "mistedo_lb_backend_services" "all" {}

resource "mistedo_lb_route" "app" {
  name             = "my-app"
  hostname         = "app.example.com"
  cloud_gateway_id = data.mistedo_load_balancers.all.gateways[0].id

  services = [
    {
      service_id = data.mistedo_lb_backend_services.all.services[0].id
    },
  ]

  healthcheck = {
    path     = "/"
    scheme   = "http"
    port     = 8080
    interval = 15
    timeout  = 3
    method   = "GET"
  }
}
```

Pick **gateway** and **service** indices (`[0]`, `[1]`, …) that match your environment, or use `for` / `lookup` patterns in your own modules.

---

## Example: weighted split (same service, different weights)

```hcl
resource "mistedo_lb_route" "weighted" {
  name             = "weighted"
  hostname         = "weighted.example.com"
  cloud_gateway_id = data.mistedo_load_balancers.all.gateways[0].id

  services = [
    {
      service_id   = data.mistedo_lb_backend_services.all.services[0].id
      balance_type = "weighted"
      weight       = 1
    },
    {
      service_id   = data.mistedo_lb_backend_services.all.services[0].id
      balance_type = "weighted"
      weight       = 3
    },
  ]

  healthcheck = {
    path    = "/"
    method  = "GET"
    headers = { Auth = "example" }
  }
}
```

---

## Arguments (summary)

* `name` — (Required) Human-friendly route name in the API.
* `hostname` — (Required) **Public hostname** clients use.
* `cloud_gateway_id` — (Required) Numeric gateway id from the data source.
* `services` — (Required) List of `{ service_id, weight?, balance_type? }`.
* `path` — (Optional) URL prefix; default `/`.
* `target_port` — (Optional) Backend port; default `80`.
* `ip_version` — (Optional) `4` or `6`.
* `insecure` / `tls_termination` / `certificate_id` — (Optional) TLS behaviour; see API for valid combinations.
* `healthcheck` — (Optional) If set, enables health checks. Nested: `path`, `scheme`, `hostname`, `port`, `interval`, `timeout`, `method`, `follow_redirects`, `headers` (map).
* `source_proto` / `destination_proto` / `labels` — (Optional) When the API requires them.

---

## Attributes

* `id` — Route id in the API (string).
* `owner` — Owner if returned.
* `gateway_name` — Resolved gateway name.

---

## Import

```shell
terraform import mistedo_lb_route.app 123
```

Use the **numeric route id** from the API.

---

## Notes

* The API may allow **duplicate** hostname/path pairs; enforce uniqueness in Terraform if that matters to you.
* Default HTTP client timeouts apply; there is no `timeouts` block on this resource.
