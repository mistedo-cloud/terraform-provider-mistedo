---
page_title: "Provider: Mistedo"
description: |-
  Configure the Mistedo Terraform provider: OAuth2 authentication, regions, and resources for DNS, networking, compute, load balancing, and object storage.
---

# Mistedo Provider

Use this provider to manage **[Mistedo](https://mistedo.cloud)** resources with **Terraform**: DNS zones and records, **private networks**, **firewall groups** and rules, **instance groups**, **application load balancer** routes and certificates, **object storage** (S3), and **iSCSI** block storage.

Authentication uses your **cloud console login** (username and password). The provider obtains an OAuth2 access token and calls regional APIs at:

`https://api.<location>.mistedo.by`

For example, `location = "dev"` uses `https://api.dev.mistedo.by`.

## Concepts

| Concept | Meaning |
|---------|--------|
| **Account** | Tenant id (e.g. `ha001`). Sent on requests so the API scopes data to your tenant. |
| **Location** | Region or environment code (`dev`, …). Part of the API hostname. |
| **Role** | API role (e.g. `owner`). With **account** it forms `account.role` used in headers. |
| **Auth URL** | Base URL for the identity server (includes `/realms`). Production defaults to `https://auth.mistedo.by/realms`. For **dev**, set **`auth_url`** to `https://auth.dev.mistedo.by/realms` if token requests fail. |

## Example configuration

Minimal provider block. Replace placeholders with your values. For **dev**, keep `auth_url` as shown.

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
  password = var.mistedo_password

  account  = "ha001"
  location = "dev"
  role     = "owner"

  auth_url    = "https://auth.dev.mistedo.by/realms"
  auth_realm  = "master"
  auth_client = "cloud-console"
}
```

You can set the same values via **environment variables** (see below). Values in the `provider` block override environment variables when both are set.

## Provider arguments

### Required

| Argument | Description |
|----------|-------------|
| `username` | Login name (often an email). |
| `password` | Login password (use a `sensitive` variable). |
| `account` | Account id (`x-auth-account`). |
| `location` | Region code → `api.<location>.mistedo.by`. |
| `role` | Role name (`x-auth-role`). With `account` builds `x-auth-group` and, for compute APIs, **`x-miq-group`**. |

### Optional (authentication)

| Argument | Default / notes |
|----------|-----------------|
| `auth_url` | If empty, the client uses built-in production defaults. Set explicitly for **dev** when needed. You may omit the trailing `/realms`; the provider can add it. |
| `auth_realm` | Default `master` when unset. |
| `auth_client` | Default **`cloud-console`** (same public client as the cloud UI for password grant). |

### Environment variables

| Variable | Maps to |
|----------|---------|
| `MISTEDO_USERNAME` | `username` |
| `MISTEDO_PASSWORD` | `password` |
| `MISTEDO_ACCOUNT` | `account` |
| `MISTEDO_LOCATION` | `location` |
| `MISTEDO_ROLE` | `role` |

There are no environment variables for `auth_url`, `auth_realm`, or `auth_client`; set those in HCL if needed.

## Request headers

Every API call includes:

- `Authorization: Bearer <token>`
- `x-auth-account`, `x-auth-role`, `x-auth-group` (group = `<account>.<role>`)

**Compute** APIs (private networks, security groups, instance groups, etc.) also send **`x-miq-group`** with the same `<account>.<role>` value. The provider sets this automatically.

**Storage** APIs (`/api/storage/v2/...`) use the same JWT and **`x-auth-account`** / **`x-auth-role`** headers; they do **not** use `x-miq-group`.

## Resources

| Resource | Purpose |
|----------|---------|
| [`mistedo_dns_zone`](resources/dns_zone.md) | DNS **hosted zone**. |
| [`mistedo_dns_record`](resources/dns_record.md) | **DNS record** (A, CNAME, MX, …). |
| [`mistedo_network`](resources/network.md) | **Private network** with one subnet. |
| [`mistedo_vpc_route`](resources/vpc_route.md) | **Static route** on the tenant default router. |
| [`mistedo_security_group`](resources/security_group.md) | **Firewall group** (rules added separately). |
| [`mistedo_security_group_rule`](resources/security_group_rule.md) | One **firewall rule** in a group. |
| [`mistedo_instance_group`](resources/instance_group.md) | **Instance group** (catalog order, multiple VMs). |
| [`mistedo_lb_route`](resources/lb_route.md) | **HTTP(S) route** on the platform load balancer. |
| [`mistedo_lb_certificate`](resources/lb_certificate.md) | **TLS certificate** for HTTPS routes. |
| [`mistedo_s3_user`](resources/s3_user.md) | **S3 user** (Ceph RGW keys). |
| [`mistedo_s3_bucket`](resources/s3_bucket.md) | **S3 bucket**. |
| [`mistedo_iscsi_disk`](resources/iscsi_disk.md) | **iSCSI block disk**. |
| [`mistedo_iscsi_client`](resources/iscsi_client.md) | **iSCSI client** (initiator / CHAP). |

## Data sources

| Data source | Returns |
|-------------|---------|
| [`mistedo_load_balancers`](data-sources/load_balancers.md) | Load balancer **gateways** (ids for routes). |
| [`mistedo_lb_backend_services`](data-sources/lb_backend_services.md) | **Backend services** for routes. |
| [`mistedo_instance_template`](data-sources/instance_template.md) | **VM catalog template** by name and version. |
| [`mistedo_s3_pools`](data-sources/s3_pools.md) | **S3 storage pools** (names for `mistedo_s3_user.pool_name`). |

## Troubleshooting

1. **`401` / `invalid_client` on non-production** — Check **`auth_url`** and realm first.
2. **Security group** first create can take **one to two minutes** (platform default rules, then the provider clears them).
3. **Provider docs** in the Registry mirror each resource page: start from the tables above and open the linked page for arguments, attributes, and import syntax.
