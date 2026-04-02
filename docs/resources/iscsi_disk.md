---
page_title: "mistedo_iscsi_disk Resource - terraform-provider-mistedo"
subcategory: "Storage"
description: |-
  iSCSI block disk (volume) via Storage API v2.
---

# mistedo_iscsi_disk (Resource)

Creates and manages an **iSCSI disk** (block volume) in your account. The disk is placed in the **iSCSI target configuration** that matches the storage **pool** you choose (`pool_name`). Target configs are provisioned by the platform; this resource does not create configs.

## Example

```hcl
resource "mistedo_iscsi_disk" "data" {
  name      = "app-disk-1"
  owner     = "ops@example.com"
  size_gb   = 100
  pool_name = "nvme-data"
}
```

## Arguments

- `name` (Required) — Disk name.
- `owner` (Required) — Owner email.
- `size_gb` (Required) — Size in GiB (minimum `1`).
- `pool_name` (Required) — Pool name from `GET /api/storage/v2/pools?type=iscsi`. Changing it forces replacement (new config/target).

## Attributes

| Name | Description |
|------|-------------|
| `id` | Numeric disk id from the Storage API (string). |
| `config_id` | iSCSI config id the disk belongs to. |
| `target_iqn` | Target IQN from the config (read-only). |
| `config_name` | Config file name from the API (read-only). |

## Import

```bash
terraform import mistedo_iscsi_disk.data <disk_id>
```

`disk_id` is the integer id returned when the disk was created.
