---
page_title: "mistedo_dns_zone Resource - Mistedo Terraform Provider"
subcategory: "DNS"
description: |-
  Creates a public DNS hosted zone on Mistedo (similar to a Route 53 public hosted zone).
---

# mistedo_dns_zone (Resource)

A **hosted zone** holds DNS records for one domain (e.g. `example.com`). After creation, add [`mistedo_dns_record`](dns_record.md) resources for names like `www` or `mail`.

The zone is scoped to your provider **account** and **role**; you do not repeat account on the resource.

## Example

```hcl
resource "mistedo_dns_zone" "main" {
  name = "app.example.com"
}
```

Use a name **without** a trailing dot (`example.com`, not `example.com.`).

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | Yes | **FQDN** of the zone. Changing it forces replacement. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Same as `name` (zone FQDN). |
| `account` | Account metadata from the API, if returned. |
| `owner` | Owner metadata (e.g. email), if returned. |

## Import

```shell
terraform import mistedo_dns_zone.main 'example.com'
```

Use the **exact** zone FQDN as in the API.

## Notes

### Naming on shared environments (e.g. dev)

New zones often must live under your account subdomain:

**`something.<account>.<location>.mistedo.by`**

Example: `tf-e2e.ha001.dev.mistedo.by` for account `ha001` and location `dev`. Names that break delegation may be **rejected** by the API.

### General

- Renaming a zone in place is **not** supported; changing `name` replaces the resource.
- If creation fails with **5xx**, try a valid delegated name, or create the zone in the UI and manage only **records** in Terraform.
- If the zone is deleted outside Terraform, the next **plan** will propose to recreate it (or remove it from state).
