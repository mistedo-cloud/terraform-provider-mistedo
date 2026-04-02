---
page_title: "mistedo_dns_zone Resource - terraform-provider-mistedo"
subcategory: "DNS"
description: |-
  Creates a public DNS hosted zone on Mistedo (like Route 53 public hosted zone).
---

# mistedo_dns_zone (Resource)

A **hosted zone** is a **container for DNS records** for one domain name (e.g. `example.com`). After you create it, you add [`mistedo_dns_record`](dns_record.md) resources for `www`, `mail`, etc.

The zone is tied to your **account** and **role** from the [provider](../index.md); you do not pass account again on the resource.

---

## Example

```hcl
resource "mistedo_dns_zone" "main" {
  name = "app.example.com"
}
```

Use a name **without** a trailing dot (`example.com`, not `example.com.`).

---

## Arguments

* `name` — (Required) **FQDN** of the zone. Changing it **forces replacement** (new zone).

---

## Attributes

* `id` — Same as `name` (the zone FQDN).
* `account` — Account metadata from the API, if returned.
* `owner` — Owner metadata (e.g. email), if returned.

---

## Import

```shell
terraform import mistedo_dns_zone.main 'example.com'
```

Use the **exact** zone FQDN as in the API.

---

## Important: naming on Mistedo (dev)

On shared environments such as **dev**, new zones often must live **under your account subdomain**, for example:

**`something.<account>.<location>.mistedo.by`**

Example: `tf-e2e.ha001.dev.mistedo.by` for account `ha001` and location `dev`.

Names that break delegation rules may be **rejected** by the API. You can still manage **records** in zones that already exist for your account (see API / console).

If creation fails with **5xx**, try a valid delegated name, or create the zone in the UI and manage only **records** in Terraform.

---

## Other notes

* Renaming a zone in place is **not** supported; Terraform will replace the resource if `name` changes.
* If someone deletes the zone outside Terraform, the next **plan** will offer to create it again (or remove it from state).
