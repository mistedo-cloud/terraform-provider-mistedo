# Compute examples

Standalone Terraform roots for **ManageIQ compute**: catalog **VM templates** and **instance group** (service) orders. Run **`terraform init`** / **`apply`** inside each folder.

| Example | What you learn |
|---------|----------------|
| [`instance-template-lookup`](instance-template-lookup/) | **`data.mistedo_instance_template`** — resolve **`template_id`** / **`full_name`** by **`name`** and optional **`version`**. |
| [`instance-group-with-network`](instance-group-with-network/) | **`mistedo_network`** + **`mistedo_security_group`** (+ SSH rule) + data template + **`mistedo_instance_group`**; **`subnet`** = **`canonical_name`**; **`boot_disk`** as a nested object. |

**Resource:** `mistedo_instance_group`  
**Data source:** `mistedo_instance_template`

DNS, firewall-only, VPC-without-VMs, and ALB samples stay under **[`../networking/`](../networking/README.md)**.

Pick a **catalog template** that exists in your region before apply (`template_name` / `template_version`).

**Credentials:** copy **`terraform.tfvars.example`** → **`terraform.tfvars`**. For **dev**, keep **`auth_url = "https://auth.dev.mistedo.by/realms"`** in the provider block.

**Local provider:** `make install` and filesystem mirror / `dev_overrides` as in the repository root README.
