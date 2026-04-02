---
page_title: "mistedo_s3_user Resource - terraform-provider-mistedo"
subcategory: "Storage"
description: |-
  Ceph RGW S3 user (technical principal with S3/Swift keys) via Storage API v2.
---

# mistedo_s3_user (Resource)

Creates a **technical S3 user** on the platform’s **Ceph RGW** object storage (Storage HTTP API **v2** only). The API returns **S3 access/secret keys** once; they are stored in Terraform state (mark the state as sensitive).

## Example

```hcl
resource "mistedo_s3_user" "app" {
  name        = "myapp"
  pool_name   = "nvme"
  owner       = "ops@example.com"
  description = "Application object storage"

  quota_buckets       = 20
  quota_data_size_mb  = 4096
  quota_objects       = 100000
}
```

## Arguments

- `name` (Required) — Short logical name; the API prefixes it with the account id.
- `pool_name` (Required) — Pool name from `GET /api/storage/v2/pools?type=s3` (e.g. `nvme`), or from the [`mistedo_s3_pools`](../data-sources/s3_pools.md) data source. Matching is case-insensitive; the provider resolves the numeric id.
- `owner` (Required) — Owner email.
- `description` (Optional) — Free-form description.
- `quota_buckets`, `quota_data_size_mb`, `quota_objects` (Optional) — User quotas. Omit or use `-1` for unlimited (same as the API).

## Attributes

| Name | Description |
|------|-------------|
| `pool_id` | Numeric pool id after resolution (read-only). |
| `full_name` | Full Ceph user id (e.g. `ha001$myapp`). |
| `s3_access_key` / `s3_secret_key` | S3 credentials (sensitive). |
| `swift_secret_key` | Swift credential if returned. |
| `usage_*` | Read-only usage from the API. |

## Import

```bash
terraform import mistedo_s3_user.app <numeric_id>
```

The id is the integer from `GET /api/storage/v2/s3/users/{id}`.
