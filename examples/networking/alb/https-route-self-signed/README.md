# ALB — HTTPS route + self-signed cert + healthcheck

Combines:

* **`mistedo_lb_certificate`** (required PEMs + generated key).
* **`mistedo_lb_route`** with **`certificate_id`** and an optional **`healthcheck`** block (`path`, `scheme`, `method`, `interval`, `timeout`).

**`target_port`** is set to **443** for a typical TLS backend; adjust if your service listens elsewhere.

If the API returns **422** on TLS fields, add or change **`tls_termination`** / **`insecure`** on the route per your environment (see ALB API docs).

Set **`cloud_gateway_id`** / **`backend_service_id`** from **[`../discover-backends`](../discover-backends/)** outputs.
