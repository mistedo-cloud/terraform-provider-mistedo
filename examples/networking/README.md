# Networking examples

These Terraform roots cover **Mistedo Networking**: public **DNS**, **firewall** (security groups), tenant **VPC** (private networks and static routes), and the **application load balancer** (Traefik Manager). **Compute** examples (**instance template**, **instance group**) live under **[`../compute/`](../compute/README.md)**.

Each folder is a **standalone module** (`terraform init` / `apply` inside that folder).

---

## DNS

| Example | What you learn |
|---------|----------------|
| [`dns/basic`](dns/basic/) | Hosted zone + **A** record with **required fields only** (`ttl` uses the provider default). |
| [`dns/mx-mail`](dns/mx-mail/) | Same pattern + **MX** with optional **`priority`**. |

**Resources:** `mistedo_dns_zone`, `mistedo_dns_record`

---

## Firewall

| Example | What you learn |
|---------|----------------|
| [`firewall/empty-group`](firewall/empty-group/) | **`mistedo_security_group`** with **`name` only** (empty group for later rules). |
| [`firewall/ingress-rules`](firewall/ingress-rules/) | Group + **`mistedo_security_group_rule`** with typical optional fields (SSH + HTTPS, scoped CIDR). |

**Resources:** `mistedo_security_group`, `mistedo_security_group_rule`

---

## VPC (private networks & routes)

| Example | What you learn |
|---------|----------------|
| [`vpc/private-network-minimal`](vpc/private-network-minimal/) | **`mistedo_network`**: **`name`** + **`cidr`** only. |
| [`vpc/private-network-dns`](vpc/private-network-dns/) | Network + optional **`dns_nameservers`**. |
| [`vpc/static-route`](vpc/static-route/) | **`mistedo_vpc_route`** (`destination` / `nexthop`; **`router_id`** is computed). |

**Resources:** `mistedo_network`, `mistedo_vpc_route`

---

## ALB (routes & certificates)

| Example | What you learn |
|---------|----------------|
| [`alb/discover-backends`](alb/discover-backends/) | **Data sources only:** `mistedo_load_balancers`, `mistedo_lb_backend_services` → copy ids for other examples. |
| [`alb/http-route-minimal`](alb/http-route-minimal/) | **`mistedo_lb_route`** with **minimal required** fields + one backend. |
| [`alb/certificate-upload`](alb/certificate-upload/) | **`mistedo_lb_certificate`** with **required PEMs** (self-signed via `hashicorp/tls`). |
| [`alb/https-route-self-signed`](alb/https-route-self-signed/) | Cert upload + **HTTPS** route with **`certificate_id`** and **`healthcheck`**. |

**Resources:** `mistedo_lb_route`, `mistedo_lb_certificate`  
**Data sources:** `mistedo_load_balancers`, `mistedo_lb_backend_services`

For **`http-route-minimal`** and **`https-route-self-signed`**, run **`discover-backends`** first and copy **`cloud_gateway_id`** / **`service_id`** from `terraform output`.

---

## Suggested order

1. **`alb/discover-backends`** — if you use ALB examples.  
2. Pick one example per service you need (DNS / firewall / VPC / ALB).  
3. Adjust names, CIDRs, and hostnames to your tenant before apply.  
4. For **VM templates / instance groups**, see **[`../compute/README.md`](../compute/README.md)**.
