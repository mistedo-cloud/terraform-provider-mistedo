provider "mistedo" {
  username = var.mistedo_username
  password = var.mistedo_password
  account  = var.mistedo_account
  location = var.mistedo_location
  role     = var.mistedo_role
  auth_url = var.mistedo_auth_url
}

data "mistedo_s3_pools" "available" {}

# Pool name: set s3_pool_name in tfvars, or pick from data.mistedo_s3_pools.available (see output s3_pool_names).
resource "mistedo_s3_user" "app" {
  name        = var.s3_user_short_name
  pool_name   = var.s3_pool_name
  owner       = var.s3_owner_email
  description = "Example S3 user from Terraform"

  quota_buckets      = 10
  quota_data_size_mb = 512
  quota_objects      = 5000
}

resource "mistedo_s3_bucket" "data" {
  name      = var.bucket_short_name
  user_name = mistedo_s3_user.app.full_name

  quota_data_size_mb = 256
  quota_objects      = 1000
}

output "s3_pool_names" {
  description = "Pool names from the API — use one as mistedo_s3_user.pool_name."
  value       = [for p in data.mistedo_s3_pools.available.pools : p.name]
}
