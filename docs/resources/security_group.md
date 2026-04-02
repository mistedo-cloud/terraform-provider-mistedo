---
page_title: "mistedo_security_group Resource - Mistedo Terraform Provider"
subcategory: "Compute"
description: |-
  Creates and deletes a compute firewall group; default rules are cleared so access is managed with mistedo_security_group_rule.
---

# mistedo_security_group (Resource)

A **security group** is a **named firewall container**. Attach [`mistedo_security_group_rule`](security_group_rule.md) resources to allow ingress or egress traffic.

On create, the platform may briefly add **two automatic “allow all egress”** rules (IPv4 and IPv6) **after a short delay**. The provider **waits**, then **removes all rules** so the group starts **empty** and only rules you declare in Terraform apply.

Create and delete run as **asynchronous jobs**; the provider waits until each finishes (up to about **15 minutes** per operation).

## Example

```hcl
resource "mistedo_security_group" "app" {
  name = "myapp_fw"
}

output "group_id" {
  value = mistedo_security_group.app.id
}

output "group_name_in_api" {
  value = mistedo_security_group.app.canonical_name
}
```

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | Yes | Short name to create (e.g. `myapp_fw`). The platform usually **prefixes** to **`<location>_<account>_<name>`**. Terraform **keeps `name` in state** as you wrote. The API name is in **`canonical_name`**. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Opaque **cloud id** of the group (use in APIs and in `security_group_id` on rules). |
| `canonical_name` | Full name from the API (with prefix). Used when **deleting** the group. |
| `ems_ref` | Internal **ManageIQ** reference (UUID). Used internally for the rule API. |

## Import

You need the numeric **cloud id** from the UI or API:

```shell
terraform import mistedo_security_group.app 81000000000108
```

## Notes

- **First apply can be slow** — waiting for default rules and stripping them is expected.
- Wrong **auth** settings (e.g. missing `auth_url` for dev) cause token errors before any resource runs; fix the [provider](../index.md) first.
- The provider discovers a **NetworkManager** provider id once per run, then calls `/api/compute/v1/...` with **`x-miq-group`** and **`x-auth-*`** headers.
