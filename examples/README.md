# Examples

Runnable samples are split by area:

- **[`networking/`](networking/)** — **DNS**, **firewall**, **VPC**, **ALB** (Traefik Manager).
- **[`compute/`](compute/)** — **`mistedo_instance_template`** (data) and **`mistedo_instance_group`** (orders).

Open **[`networking/README.md`](networking/README.md)** or **[`compute/README.md`](compute/README.md)** for each catalog.

**Credentials:** copy each example’s `terraform.tfvars.example` → `terraform.tfvars` (do not commit secrets). For **dev**, keep `auth_url = "https://auth.dev.mistedo.by/realms"` in the provider block.

**Local provider:** build with `make install` and use `dev_overrides` as in the repository root README.
