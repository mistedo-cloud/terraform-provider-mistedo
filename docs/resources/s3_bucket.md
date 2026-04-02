---
page_title: "mistedo_s3_bucket Resource - Mistedo Terraform Provider"
subcategory: "Storage"
description: |-
  S3 bucket on Ceph RGW (Storage API v2).
---

# mistedo_s3_bucket (Resource)

Creates an **S3 bucket** owned by an existing [`mistedo_s3_user`](s3_user.md). The canonical identifier is the **`path`** (`account/short-name`).

## Example

```hcl
resource "mistedo_s3_bucket" "data" {
  name      = "app-data"
  user_name = mistedo_s3_user.app.full_name

  quota_data_size_mb = 1024
  quota_objects      = 50000
}
```

**Unlimited** per-bucket quotas: **omit** both quota attributes, or set **`quota_data_size_mb = -1`** and/or **`quota_objects = -1`** (Storage API semantics). If you omit them, the API may return `-1` stored as **unset** (`null`) in state; if you set `-1` explicitly, state keeps **`-1`** so config and state align.

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | Yes | Bucket short name; combined with user for **`path`**. |
| `user_name` | Yes | Owning user — typically `mistedo_s3_user.<n>.full_name`. |
| `quota_data_size_mb` | No | Data quota (MiB); `-1` or omit for unlimited per API rules. |
| `quota_objects` | No | Object count quota; `-1` or omit for unlimited. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Full bucket path `account/short-name` (unique in storage; same meaning as `path`). |
| `path` | Canonical path from the API (same as `id`). |

## Import

```shell
terraform import mistedo_s3_bucket.data ha001/app-data
```

Import id is the full **`path`** from the API (use the plain path in the CLI; URLs may escape `/` as `%2F`).

## Notes

- Create the [`mistedo_s3_user`](s3_user.md) first; the bucket references **`user_name`**.
