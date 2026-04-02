---
page_title: "mistedo_s3_user Resource - Mistedo Terraform Provider"
subcategory: "Storage"
description: |-
  Ceph RGW S3 user (access and secret keys) via Storage API v2.
---

# mistedo_s3_user (Resource)

Creates a **technical S3 user** on **Ceph RGW** object storage (Storage HTTP API **v2**). The API returns **S3 access/secret keys** once; they are stored in Terraform state (treat state as **sensitive**).

## Example

```hcl
resource "mistedo_s3_user" "app" {
  name        = "myapp"
  pool_name   = "nvme"
  owner       = "ops@example.com"
  description = "Application object storage"

  quota_buckets      = 20
  quota_data_size_mb = 4096
  quota_objects      = 100000
}
```

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | Yes | Short logical name; the API may prefix with the account id. |
| `pool_name` | Yes | Pool name from [`mistedo_s3_pools`](../data-sources/s3_pools.md) or `GET /api/storage/v2/pools?type=s3`. Matching is case-insensitive; the provider resolves the numeric id. |
| `owner` | Yes | Owner email. |
| `description` | No | Free-form description. |
| `quota_buckets` | No | Quota; omit or use `-1` for unlimited (API semantics). |
| `quota_data_size_mb` | No | Quota; omit or `-1` for unlimited. |
| `quota_objects` | No | Quota; omit or `-1` for unlimited. |

## Attributes

| Name | Description |
|------|-------------|
| `pool_id` | Numeric pool id after resolution (read-only). |
| `full_name` | Full Ceph user id (e.g. `ha001$myapp`). |
| `s3_access_key` | S3 access key (**sensitive**). |
| `s3_secret_key` | S3 secret key (**sensitive**). |
| `swift_secret_key` | Swift credential if returned (**sensitive**). |

## Import

```shell
terraform import mistedo_s3_user.app <numeric_id>
```

The id is the integer from `GET /api/storage/v2/s3/users/{id}`.

## Notes

- Prefer selecting **`pool_name`** from the [`mistedo_s3_pools`](../data-sources/s3_pools.md) data source instead of hard-coding.
