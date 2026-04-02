---
page_title: "mistedo_s3_pools Data Source - Mistedo Terraform Provider"
subcategory: "Storage"
description: |-
  Lists S3-capable storage pools; use pool names when creating mistedo_s3_user.
---

# mistedo_s3_pools (Data Source)

Returns **S3 storage pools** for your account (`GET /api/storage/v2/pools?type=s3`).

Use a pool **`name`** as **`pool_name`** on [`mistedo_s3_user`](../resources/s3_user.md).

## Example

```hcl
data "mistedo_s3_pools" "all" {}

resource "mistedo_s3_user" "app" {
  name      = "myapp"
  pool_name = data.mistedo_s3_pools.all.pools[0].name
  owner     = "ops@example.com"
}

output "pool_names" {
  value = [for p in data.mistedo_s3_pools.all.pools : p.name]
}
```

In production, select a pool **by `name`** (variable, `one()` + filter, or similar) instead of hard-coding `[0]`.

## Arguments

This data source has **no** configuration arguments.

## Attributes

| Name | Description |
|------|-------------|
| `pools` | Sorted by numeric **`id`**, then **`name`**. Each object is described below. |

### `pools` objects

| Field | Description |
|-------|-------------|
| `id` | Numeric pool id (matches computed `pool_id` on [`mistedo_s3_user`](../resources/s3_user.md) after create). |
| `name` | Value for [`mistedo_s3_user.pool_name`](../resources/s3_user.md). |
| `klass` | Pool class / tier from the API. |
| `type` | Pool type (`s3` for this data source). |

## Notes

- Matching against [`mistedo_s3_user`](../resources/s3_user.md) is **case-insensitive** for `pool_name`.
