# Object storage (S3 / Ceph RGW)

Uses **Storage API v2** (`/api/storage/v2`): [`mistedo_s3_user`](../../docs/resources/s3_user.md) and [`mistedo_s3_bucket`](../../docs/resources/s3_bucket.md).

Set **`s3_pool_name`** in `terraform.tfvars`, or run **`terraform apply`** once and read **`s3_pool_names`** from the output (from [`mistedo_s3_pools`](../../docs/data-sources/s3_pools.md)).

Set provider credentials via `terraform.tfvars` (not committed) or `MISTEDO_*` env vars.

```bash
terraform init
terraform apply
```

After apply, use the S3 endpoint from your account quota response (e.g. `https://s3.dev.mistedo.by`) with the computed access/secret keys (see `terraform output` if you add outputs).
