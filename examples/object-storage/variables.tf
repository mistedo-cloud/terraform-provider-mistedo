variable "mistedo_username" { type = string }
variable "mistedo_password" { type = string sensitive = true }
variable "mistedo_account" { type = string }
variable "mistedo_location" { type = string }
variable "mistedo_role" { type = string }
variable "mistedo_auth_url" {
  type        = string
  description = "e.g. https://auth.dev.mistedo.by/realms for dev"
}

variable "s3_pool_name" {
  type        = string
  description = "S3 pool name from Storage API (GET .../pools?type=s3), e.g. nvme."
}

variable "s3_user_short_name" {
  type        = string
  description = "Short name; API prefixes with account (e.g. ha001$<name>)."
}

variable "s3_owner_email" {
  type = string
}

variable "bucket_short_name" {
  type = string
}
