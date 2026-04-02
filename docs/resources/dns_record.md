---
page_title: "mistedo_dns_record Resource - terraform-provider-mistedo"
subcategory: "DNS"
description: |-
  Creates and updates a DNS record (A, CNAME, MX, etc.) inside a hosted zone.
---

# mistedo_dns_record (Resource)

A **DNS record** is one row in DNS: a **name** (like `www`), a **type** (`A`, `CNAME`, …), and **content** (IP, hostname, text for TXT, etc.).

The record belongs to a **zone** you already have — either created with [`mistedo_dns_zone`](dns_zone.md) or an existing zone in your account. Pass the zone’s **FQDN** in the `zone` argument.

---

## Examples

### A record for a web server

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

### MX record (mail)

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

`@` is a common convention for the **zone apex** (the bare domain); the provider sends what the API accepts for your zone.

---

## Arguments

* `zone` — (Required) Hosted zone **FQDN** (usually `mistedo_dns_zone.<n>.name`). Changing **zone** replaces the record.
* `name` — (Required) **Relative** owner name (`www`, `api`, …). Changing **name** replaces the record.
* `type` — (Required) One of: **`A`**, **`AAAA`**, **`CNAME`**, **`TXT`**, **`SRV`**, **`MX`**, **`NS`**. Case in config is normalized. Changing **type** replaces the record.
* `content` — (Required) **RDATA**: IP, hostname, TXT string, etc., as required by the type.
* `ttl` — (Optional) Seconds. Default **300** if omitted.
* `priority` — (Optional) For **MX** and **SRV** (0–65535).
* `weight` / `port` — (Optional) For **SRV** when needed.

---

## Attributes

* `id` — Composite: `<zone>/<record_path>` (opaque path segment from the API).
* `record_path` — API identifier for updates/deletes (read-only).
* `group` — Optional API grouping (read-only).

---

## How updates work

The HTTP API does not offer a small **PATCH** for records. The provider **updates** by **delete + create**. If delete succeeds but create fails, fix the config and run **apply** again.

Changing **`zone`**, **`name`**, or **`type`** forces **destroy + create** instead of that path.

---

## Import

```text
<zone_fqdn>/<record_path>
```

`record_path` comes from the API or from `terraform state show` after create.

```shell
terraform import 'mistedo_dns_record.www' 'example.com/x1.www'
```

---

## Errors

Auth or permission problems usually show as **401** or **403**. Invalid data often returns **4xx** with a short message from the API body.
