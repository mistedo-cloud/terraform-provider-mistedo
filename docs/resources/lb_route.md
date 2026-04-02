---
page_title: "mistedo_lb_route Resource - Mistedo Terraform Provider"
subcategory: "ALB"
description: |-
  HTTP(S) route on the Mistedo application load balancer: hostname, backends, optional health check and TLS.
---

# mistedo_lb_route (Resource)

Defines how traffic for a **hostname** (and path) on a platform **load balancer** is sent to **backend services** and how **health checks** run.

You need:

1. A **gateway** id from [`mistedo_load_balancers`](../data-sources/load_balancers.md) → `cloud_gateway_id`.
2. One or more **backend service** ids from [`mistedo_lb_backend_services`](../data-sources/lb_backend_services.md) → `services`.

Updates send a **full** JSON body to the API; do not rely on partial merges outside Terraform.

## Example

### Simple route with health check

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

Pick gateway and service entries that match your environment (not always `[0]`).

### Weighted split

```hcl
resource "mistedo_lb_route" "weighted" {
  name               = "weighted"
  hostname           = "weighted.example.com"
  cloud_gateway_id   = data.mistedo_load_balancers.all.gateways[0].id

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

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | Yes | Route name in the API. |
| `hostname` | Yes | **Public hostname** clients use. |
| `cloud_gateway_id` | Yes | Numeric gateway id from the data source. |
| `services` | Yes | List of `{ service_id, weight?, balance_type? }`. |
| `path` | No | URL prefix; default `/`. |
| `target_port` | No | Backend port; default `80`. |
| `ip_version` | No | `4` or `6`. |
| `insecure` | No | TLS behaviour; see API for valid combinations with other TLS fields. |
| `tls_termination` | No | TLS behaviour. |
| `certificate_id` | No | Certificate id when using TLS options the API expects. |
| `healthcheck` | No | If set, enables health checks. Nested: `path`, `scheme`, `hostname`, `port`, `interval`, `timeout`, `method`, `follow_redirects`, `headers` (map). |
| `source_proto` | No | When the API requires it. |
| `destination_proto` | No | When the API requires it. |
| `labels` | No | When the API requires them. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Route id in the API (string). |
| `owner` | Owner if returned. |
| `gateway_name` | Resolved gateway name. |

## Import

```shell
terraform import mistedo_lb_route.app 123
```

Use the **numeric route id** from the API.

## Notes

- The API may allow **duplicate** hostname/path pairs; enforce uniqueness in your modules if needed.
- Default HTTP client timeouts apply; this resource has no `timeouts` block.
