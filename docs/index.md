---
page_title: "Provider: mistedo"
description: |-
  Configure the Mistedo Terraform provider: authentication, regions, and links to DNS, private networks, compute security groups, and load balancer resources.
---

# Mistedo Provider

This provider lets you manage parts of your **[Mistedo](https://mistedo.cloud)** cloud with **Terraform**: DNS zones and records, **private networks**, **firewall groups** (security groups) and rules, and **application load balancer** routes and certificates.

You **log in with the same style of account** the cloud console uses: **Keycloak** username and password. The provider requests a token and calls regional APIs at:

`https://api.<location>.mistedo.by`

For example, if `location = "dev"`, API calls go to `https://api.dev.mistedo.by`.

---

## Concepts (quick)

| Idea | Meaning |
|------|--------|
| **Account** | Your tenant id on the platform (e.g. `ha001`). Sent on every request so the API knows whose resources to use. |
| **Location** | Short code for the **region / environment** (`dev`, etc.). It becomes part of the API hostname. |
| **Role** | API role name (e.g. `owner`). Together with **account** it forms the group string `account.role` used in headers. |
| **Auth URL** | Where Keycloak lives. **Production** defaults to `https://auth.mistedo.by/realms`. For **dev**, you usually must set **`auth_url`** to `https://auth.dev.mistedo.by/realms` or token requests will fail. |

If you are new to Terraform, you only need: install Terraform, write a `.tf` file, run `terraform init` then `terraform apply`. Credentials should not be committed to Git; use a `terraform.tfvars` file (gitignored) or environment variables.

---

## Example: minimal configuration

The block below is enough to start. **Replace** the placeholder values with yours. For **dev**, keep the `auth_url` line as shown.

```hcl
terraform {
  required_providers {
    mistedo = {
      source  = "mistedo-cloud/mistedo"
      version = "~> 0.1"
    }
  }
}

provider "mistedo" {
  username = "you@example.com"
  password = "your-keycloak-password"

  account  = "ha001"
  location = "dev"
  role     = "owner"

  # Required for dev (and any environment where Keycloak is not the default host):
  auth_url    = "https://auth.dev.mistedo.by/realms"
  auth_realm  = "master"
  auth_client = "cloud-console"
}
```

Same settings can be supplied with **environment variables** (see below). Values in the `provider` block **override** env vars when both are set.

---

## Provider arguments

### Required

| Argument | Description |
|----------|-------------|
| `username` | Keycloak user (often an email). |
| `password` | Keycloak password (mark `sensitive` in variable definitions). |
| `account` | Account id (`x-auth-account`). |
| `location` | Region code → `api.<location>.mistedo.by`. |
| `role` | Role name (`x-auth-role`). With `account` builds `x-auth-group` and, for compute APIs, **`x-miq-group`**. |

### Optional (Keycloak)

| Argument | Default / notes |
|----------|-----------------|
| `auth_url` | If empty, the client uses built-in defaults (production-style). Set explicitly for **dev** (see example above). You may omit the trailing `/realms`; the provider can add it. |
| `auth_realm` | Default `master` when unset. |
| `auth_client` | Default **`cloud-console`** (same public client as the cloud UI for password grant). Change only if your IdP uses another client. |

### Environment variables

| Variable | Maps to |
|----------|---------|
| `MISTEDO_USERNAME` | `username` |
| `MISTEDO_PASSWORD` | `password` |
| `MISTEDO_ACCOUNT` | `account` |
| `MISTEDO_LOCATION` | `location` |
| `MISTEDO_ROLE` | `role` |

There are no env vars for `auth_url` / `auth_realm` / `auth_client` in the provider schema; set those in HCL if needed.

---

## What the provider sends on each request

Every API call includes:

- `Authorization: Bearer <token>`
- `x-auth-account`, `x-auth-role`, `x-auth-group` (group = `<account>.<role>`)

**Compute** (private networks, security groups, tasks) also requires **`x-miq-group`** with the same `<account>.<role>` value. The provider sets this automatically when using those APIs.

**Storage** (Ceph RGW control plane, `GET/POST /api/storage/v2/...`) uses the same JWT and **`x-auth-account`** / **`x-auth-role`** headers; it does **not** use `x-miq-group`.

---

## Resources (what you can manage)

| Resource | Plain-language purpose |
|----------|-------------------------|
| [`mistedo_dns_zone`](resources/dns_zone.md) | A **DNS zone** (like a hosted zone in AWS Route 53). |
| [`mistedo_dns_record`](resources/dns_record.md) | A **single DNS record** (A, CNAME, MX, …) inside a zone. |
| [`mistedo_network`](resources/network.md) | A **private network** with one subnet (VPC-style tenant network). |
| [`mistedo_vpc_route`](resources/vpc_route.md) | A **static VPC route** on the tenant default **network router** (`add_route` / `remove_route`). |
| [`mistedo_security_group`](resources/security_group.md) | A **firewall group** for VMs/network (empty rules after create; you add rules separately). |
| [`mistedo_security_group_rule`](resources/security_group_rule.md) | One **allow rule** (e.g. SSH from a CIDR) in a group. |
| [`mistedo_instance_group`](resources/instance_group.md) | **Instance group** (catalog service): several VMs from one template (cart order + provision wait). |
| [`mistedo_lb_route`](resources/lb_route.md) | **HTTP(S) routing**: hostname → backend services on the platform load balancer. |
| [`mistedo_lb_certificate`](resources/lb_certificate.md) | **TLS certificate** uploaded for HTTPS routes. |
| [`mistedo_s3_user`](resources/s3_user.md) | **Ceph RGW S3 user** (access/secret keys for S3-compatible object storage). |
| [`mistedo_s3_bucket`](resources/s3_bucket.md) | **S3 bucket** in that account (bucket name + owning S3 user). |
| [`mistedo_iscsi_disk`](resources/iscsi_disk.md) | **iSCSI block disk** (volume in an existing iSCSI target for a pool). |
| [`mistedo_iscsi_client`](resources/iscsi_client.md) | **iSCSI client** (initiator IQN / CHAP for attaching disks). |

## Data sources (read-only lists)

| Data source | Returns |
|-------------|---------|
| [`mistedo_load_balancers`](data-sources/load_balancers.md) | Available **load balancer gateways** (ids for routes). |
| [`mistedo_lb_backend_services`](data-sources/lb_backend_services.md) | **Backend services** you can attach to a route. |
| [`mistedo_instance_template`](data-sources/instance_template.md) | **VM catalog template** by product name and optional version (for ordering / instance groups). |
| [`mistedo_s3_pools`](data-sources/s3_pools.md) | **S3 storage pools** (names for `mistedo_s3_user.pool_name`). |

---

## Where to go next

1. Pick a resource from the table above and open its page — each page starts with a **short explanation** and a **copy-paste example**.
2. If `terraform apply` fails with **401** / **invalid_client** on a non-production environment, check **`auth_url`** first.
3. Security group **create** can take **one to two minutes** the first time (the platform adds default rules, then the provider removes them).
