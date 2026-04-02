---
page_title: "mistedo_s3_pools Data Source - terraform-provider-mistedo"
subcategory: "Storage"
description: |-
  Lists S3-capable storage pools; use pool names when creating mistedo_s3_user.
---

# mistedo_s3_pools (Data Source)

**Read-only.** Returns every **S3 storage pool** available to your account (`GET /api/storage/v2/pools?type=s3`).

Use a pool’s **`name`** as **`pool_name`** on [`mistedo_s3_user`](../resources/s3_user.md). You do not need to look up numeric ids by hand.

---

## Example

```hcl
data "mistedo_s3_pools" "all" {}

resource "mistedo_s3_user" "app" {
  name      = "myapp"
  pool_name = data.mistedo_s3_pools.all.pools[0].name # or pick by name with for-expression
  owner     = "ops@example.com"
}

output "pool_names" {
  value = [for p in data.mistedo_s3_pools.all.pools : p.name]
}
```

In production configs, prefer selecting a pool **by `name`** (for example with `one()` and a `filter` block in Terraform 1.5+, or a variable) instead of hard-coding `[0]`.

---

## Attributes

`pools` — sorted by numeric **`id`**, then **`name`**. Each object has:

| Field | Meaning |
|-------|--------|
| `id` | Numeric pool id (matches computed `pool_id` on `mistedo_s3_user` after create). |
| `name` | Value to pass to `mistedo_s3_user.pool_name`. |
| `klass` | Pool class / tier from the API. |
| `type` | Pool type (`s3` for this data source). |
