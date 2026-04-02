---
page_title: "mistedo_iscsi_disk Resource - Mistedo Terraform Provider"
subcategory: "Storage"
description: |-
  iSCSI block disk (volume) via Storage API v2.
---

# mistedo_iscsi_disk (Resource)

Creates an **iSCSI disk** (block volume) in your account. The disk is placed in the **iSCSI target configuration** for the chosen **`pool_name`**. Target configs are provisioned by the platform; this resource does not create configs.

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

| Name | Required | Description |
|------|----------|-------------|
| `name` | Yes | Disk name. |
| `owner` | Yes | Owner email. |
| `size_gb` | Yes | Size in GiB (minimum `1`). |
| `pool_name` | Yes | Pool from `GET /api/storage/v2/pools?type=iscsi`. Changing it forces replacement. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Numeric disk id from the Storage API (string). |
| `config_id` | iSCSI config id the disk belongs to. |
| `target_iqn` | Target IQN from the config (read-only). |
| `config_name` | Config file name from the API (read-only). |

## Import

```shell
terraform import mistedo_iscsi_disk.data <disk_id>
```

`disk_id` is the integer id returned at create.

## Notes

- Attach disks to clients with [`mistedo_iscsi_client`](iscsi_client.md) using the disk **`id`** and **`name`**.
