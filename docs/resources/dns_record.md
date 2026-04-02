---
page_title: "mistedo_dns_record Resource - Mistedo Terraform Provider"
subcategory: "DNS"
description: |-
  Creates and updates a DNS record (A, CNAME, MX, etc.) inside a hosted zone.
---

# mistedo_dns_record (Resource)

A **DNS record** is one row in DNS: **name** (e.g. `www`), **type** (`A`, `CNAME`, …), and **content** (IP, hostname, TXT payload, etc.).

The record belongs to an existing zone — from [`mistedo_dns_zone`](dns_zone.md) or already present in your account. Pass the zone **FQDN** in `zone`.

## Example

### A record

```hcl
resource "mistedo_dns_zone" "main" {
  name = "example.com"
}

resource "mistedo_dns_record" "www" {
  zone    = mistedo_dns_zone.main.name
  name    = "www"
  type    = "A"
  content = "203.0.113.10"
  ttl     = 300
}
```

### MX record

```hcl
resource "mistedo_dns_record" "mx" {
  zone     = mistedo_dns_zone.main.name
  name     = "@"
  type     = "MX"
  content  = "mail.example.com"
  ttl      = 3600
  priority = 10
}
```

`@` is a common convention for the **zone apex**; the provider sends what the API accepts for your zone.

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `zone` | Yes | Hosted zone **FQDN** (usually `mistedo_dns_zone.<n>.name`). Changing **zone** replaces the record. |
| `name` | Yes | **Relative** owner name (`www`, `api`, …). Changing **name** replaces the record. |
| `type` | Yes | One of **`A`**, **`AAAA`**, **`CNAME`**, **`TXT`**, **`SRV`**, **`MX`**, **`NS`**. Case is normalized. Changing **type** replaces the record. |
| `content` | Yes | **RDATA** (IP, hostname, TXT string, …) depending on type. |
| `ttl` | No | Seconds. Default **300** if omitted. |
| `priority` | No | For **MX** and **SRV** (0–65535). |
| `weight` | No | For **SRV** when needed. |
| `port` | No | For **SRV** when needed. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Composite: `<zone>/<record_path>` (opaque path from the API). |
| `record_path` | API identifier for updates and deletes (read-only). |
| `group` | Optional API grouping (read-only). |

## Import

Import id format:

```text
<zone_fqdn>/<record_path>
```

`record_path` comes from the API or from `terraform state show` after create.

```shell
terraform import 'mistedo_dns_record.www' 'example.com/x1.www'
```

## Notes

### Update behavior

The HTTP API does not expose a small **PATCH** for records. The provider **updates** by **delete + create**. If delete succeeds but create fails, fix the config and run **apply** again.

Changing **`zone`**, **`name`**, or **`type`** forces **destroy + create** instead of that path.

### Errors

Auth or permission issues usually surface as **401** or **403**. Invalid data often returns **4xx** with a message from the API body.
