---
page_title: "mistedo_instance_template Data Source - terraform-provider-mistedo"
subcategory: "Compute"
description: |-
  Looks up a VM catalog service template by product name and optional version (ManageIQ service_templates).
---

# mistedo_instance_template (Data Source)

**Read-only.** Returns one **service template** from the compute catalog (`GET /api/compute/v1/service_templates`). You need a template id when you order VMs or create an **instance group** later.

The platform stores each template under a single display string, usually **`product:version`** (for example `Ubuntu Server:24.04.1-240905`). In Terraform you pass **`name`** (the part before the first `:`) and optionally **`version`** (the rest).

---

## Case sensitivity

Filtering is **case-sensitive** on the API: `Ubuntu` and `ubuntu` are different. If lookup fails, check **capital letters** in both **`name`** and **`version`**.

---

## Example

Exact match (recommended when you know the version):

```hcl
data "mistedo_instance_template" "ubuntu" {
  name    = "Ubuntu Server"
  version = "24.04.1-240905"
}

output "template_id" {
  value = data.mistedo_instance_template.ubuntu.id
}
```

Match by substring (no `version`): the provider sends a SQL **LIKE** pattern `%%name%%`. It must match **exactly one** template. If several match, Terraform fails with an error that **lists every matching template** (`id` and full catalog `full_name`) so you can set **`version`** and/or a longer, more specific **`name`**.

```hcl
data "mistedo_instance_template" "ubuntu_loose" {
  name = "Ubuntu Server"
}
```

---

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | yes | Product segment before `:` in the catalog name. **Case-sensitive.** |
| `version` | no | Version segment after `:`; when set, the API match is **exact** on `name:version`. **Case-sensitive.** |

---

## Attributes

| Name | Description |
|------|-------------|
| `id` | Service template id (numeric string from ManageIQ). |
| `full_name` | Full catalog name from the API (typically `name:version`). |
| `guid` | Template `guid` when the API returns it. |
