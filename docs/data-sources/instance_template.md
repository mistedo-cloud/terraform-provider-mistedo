---
page_title: "mistedo_instance_template Data Source - Mistedo Terraform Provider"
subcategory: "Compute"
description: |-
  Looks up a VM catalog service template by product name and optional version (ManageIQ service_templates).
---

# mistedo_instance_template (Data Source)

Returns one **service template** from the compute catalog (`GET /api/compute/v1/service_templates`). Use the template **`id`** when ordering VMs or creating an [`mistedo_instance_group`](../resources/instance_group.md).

Catalog entries are usually **`product:version`** (e.g. `Ubuntu Server:24.04.1-240905`). In Terraform use **`name`** (text before the first `:`) and optionally **`version`** (text after).

## Example

### Exact match

```hcl
data "mistedo_instance_template" "ubuntu" {
  name    = "Ubuntu Server"
  version = "24.04.1-240905"
}

output "template_id" {
  value = data.mistedo_instance_template.ubuntu.id
}
```

### Match by name only

The provider sends a SQL **LIKE** pattern `%%name%%`. It must match **exactly one** template. If several match, Terraform fails and **lists** matching templates (`id` and `full_name`) so you can narrow **`name`** and/or set **`version`**.

```hcl
data "mistedo_instance_template" "ubuntu_loose" {
  name = "Ubuntu Server"
}
```

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | Yes | Product segment before `:` in the catalog name. **Case-sensitive.** |
| `version` | No | Version segment after `:`; when set, match is **exact** on `name:version`. **Case-sensitive.** |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Service template id (numeric string from ManageIQ). |
| `full_name` | Full catalog name from the API (typically `name:version`). |
| `guid` | Template `guid` when the API returns it. |

## Notes

### Case sensitivity

Filtering is **case-sensitive**: `Ubuntu` and `ubuntu` are different. If lookup fails, verify capitalization in **`name`** and **`version`**.
