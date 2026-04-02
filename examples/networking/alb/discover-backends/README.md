# ALB — discover gateways and services (data sources only)

Runs **`mistedo_load_balancers`** and **`mistedo_lb_backend_services`** with **no resources**, so you can copy **`cloud_gateway_id`** and **`service_id`** into **`alb-http-minimal`** or **`alb-https-self-signed`**.

If lists are empty, the account may not have Traefik Manager backends yet.
