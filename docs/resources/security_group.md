---
page_title: "mistedo_security_group Resource - terraform-provider-mistedo"
subcategory: "Compute"
description: |-
  Creates and deletes a compute firewall group (security group) on Mistedo; default rules are cleared so you manage access with mistedo_security_group_rule.
---

# mistedo_security_group (Resource)

A **security group** is a **named firewall container**: you attach **rules** to it to allow ingress or egress traffic (see [`mistedo_security_group_rule`](security_group_rule.md)).

On create, the platform may briefly add **two automatic “allow all egress” rules** (IPv4 and IPv6). Those appear **after a short delay**. The provider **waits for them**, then **removes every rule** so the group starts **empty** and only the rules you declare in Terraform apply.

Create and delete run **asynchronous jobs** on the server; the provider waits until each job finishes (up to about **15 minutes** per operation).

---

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

---

## Arguments

* `name` — (Required) Short name you want to create (e.g. `myapp_fw`). The platform usually **prefixes** it to  
  **`<location>_<account>_<name>`**.  
  Terraform **keeps `name` in state equal to what you wrote** in configuration so apply does not fail with a mismatch. The real API name is in **`canonical_name`**.

---

## Attributes

* `id` — Opaque **cloud id** of the group (use in APIs and in `security_group_id` on rules).
* `canonical_name` — Full name returned by the API (with prefix). Used when **deleting** the group.
* `ems_ref` — Internal **ManageIQ** reference (UUID). You do not set this; the provider uses it when talking to the rule API.

---

## Import

You need the numeric **cloud id** from the UI or API:

```shell
terraform import mistedo_security_group.app 81000000000108
```

---

## Practical notes

* **First apply is slow** — waiting for default rules and stripping them is normal.
* **Wrong Keycloak host** (e.g. missing `auth_url` for dev) causes token errors before any resource is created; fix the [provider](../index.md) first.
* Under the hood the provider discovers a **NetworkManager** provider id once per run, then calls `/api/compute/v1/...` with **`x-miq-group`** and **`x-auth-*`** headers.
