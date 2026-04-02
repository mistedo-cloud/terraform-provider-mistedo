# ALB — minimal HTTP route

**Required-only** on **`mistedo_lb_route`**: **`name`**, **`hostname`**, **`cloud_gateway_id`**, **`services`** (at least one **`service_id`**).

Fill **`cloud_gateway_id`** and **`backend_service_id`** from **[`../discover-backends`](../discover-backends/)** (`terraform output`).

No **`healthcheck`** block — add one in **`alb-https-self-signed`** or extend this file when you need probes.
