# DNS — zone + A record (required fields only)

1. Pick a **zone FQDN** you may create on the platform (no trailing dot).
2. Copy `terraform.tfvars.example` → `terraform.tfvars` and fill credentials.
3. `terraform init && terraform apply`

The `www` **A** record sets only **required** attributes; **`ttl`** is omitted (provider default **300**).
