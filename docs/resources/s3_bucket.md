---
page_title: "mistedo_s3_bucket Resource - terraform-provider-mistedo"
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

You can express **no per-bucket limit** (unlimited) by **omitting** both quota attributes, or by setting **`quota_data_size_mb = -1`** and/or **`quota_objects = -1`** (same as the Storage API). If you omit them, the API’s `-1` values are stored as **unset** (`null`) in state; if you set `-1` explicitly, state keeps **`-1`** so the config and state stay aligned.

## Import

```bash
terraform import mistedo_s3_bucket.data ha001/app-data
```

Import id is the full **`path`** returned by the API (slash may appear as `%2F` in URLs; use the plain path in the CLI).
