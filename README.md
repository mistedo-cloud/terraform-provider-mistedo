# Terraform Provider for Mistedo

This provider allows managing resources on the [Mistedo](https://mistedo.cloud) cloud platform using Terraform.  
Authentication is done via **Keycloak**.  
**DNS:** [`mistedo_dns_zone`](docs/resources/dns_zone.md) and [`mistedo_dns_record`](docs/resources/dns_record.md). **Compute:** [`mistedo_network`](docs/resources/network.md), [`mistedo_vpc_route`](docs/resources/vpc_route.md), [`mistedo_security_group`](docs/resources/security_group.md), [`mistedo_security_group_rule`](docs/resources/security_group_rule.md), [`mistedo_instance_group`](docs/resources/instance_group.md), data source [`mistedo_instance_template`](docs/data-sources/instance_template.md). **ALB (Traefik Manager):** [`mistedo_lb_route`](docs/resources/lb_route.md), [`mistedo_lb_certificate`](docs/resources/lb_certificate.md), data sources [`mistedo_load_balancers`](docs/data-sources/load_balancers.md), [`mistedo_lb_backend_services`](docs/data-sources/lb_backend_services.md). **Object storage (Ceph RGW):** [`mistedo_s3_user`](docs/resources/s3_user.md), [`mistedo_s3_bucket`](docs/resources/s3_bucket.md) (Storage API v2). Full provider options: [`docs/index.md`](docs/index.md).

**Examples:** [`examples/networking/README.md`](examples/networking/README.md) (DNS, firewall, VPC, ALB) and [`examples/compute/README.md`](examples/compute/README.md) (`mistedo_instance_template`, `mistedo_instance_group`).

**Local run without the registry:** follow **Building and using locally** below (`make install`, `~/.terraformrc` from [`terraform.rc.example`](terraform.rc.example), then `terraform init` / `plan` / `apply`). Start from any example under [`examples/`](examples/).

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.26 (to build the provider)

## Provider configuration

```hcl
provider "mistedo" {
  username = "your-username"
  password = "your-password"
  account  = "your-account"
  location = "your-region"
  role     = "your-role"

  # Optional (Keycloak)
  auth_url    = "https://auth.example.com/realms"
  auth_realm  = "master"           # default
  auth_client = "cloud-console" # default (Keycloak client for the cloud console)
}
```

### Environment variables

Credentials and context can be provided via environment variables instead of provider arguments:

- `MISTEDO_USERNAME`
- `MISTEDO_PASSWORD`
- `MISTEDO_ACCOUNT`
- `MISTEDO_LOCATION`
- `MISTEDO_ROLE`

### Validation

The provider validates that all required parameters (username, password, account, location, role) are set. Values from the configuration override environment variables.

## JWT token handling

Authentication uses Keycloak (OAuth2 resource owner password flow): the provider sends username and password to the Keycloak token endpoint and receives a JWT access token.

**Where the token is stored**

- The token is cached **in memory** for the lifetime of the provider process.
- Terraform runs the provider in a separate process for each operation (`plan`, `apply`, etc.), so the token is reused for all API calls within that single run and is not shared between runs.
- No token is written to disk, so credentials are not persisted.

**When a new token is requested**

- When there is no token yet.
- When the server returns `expires_in` and the token is close to expiry (refresh 60 seconds before expiry).
- After a 401 response you can call `InvalidateToken()` so the next request fetches a new token (useful when implementing resources).

This avoids requesting a token for every API call while keeping a single process run efficient and avoiding disk storage of tokens.

## Building and using locally (without registry)

Terraform by default tries to download providers from the registry. To use your locally built provider:

1. **In your Terraform config** use the source `mistedo-cloud/mistedo` (not `hashicorp/mistedo`):

```hcl
terraform {
  required_providers {
    mistedo = {
      source  = "mistedo-cloud/mistedo"
      version = "0.0.1"   # must match the version used by make install
    }
  }
}
```

2. **Configure a filesystem mirror** in the CLI config so Terraform does not contact the registry. Copy the example, then set `YOUR_HOME` to your home directory (e.g. `/Users/user1`):

```bash
cp terraform.rc.example ~/.terraformrc
# Edit ~/.terraformrc and replace YOUR_HOME with your real path
```

On Windows use `%APPDATA%\terraform.rc` and a path like `C:\Users\YourName\.terraform.d\plugins`.

3. **Build and install** the provider (this puts the binary in the mirror layout):

```bash
make build
make install
```

By default the binary is installed under `~/.terraform.d/plugins/registry.terraform.io/mistedo-cloud/mistedo/0.0.1/<os>_<arch>/`. To use another version or mirror root:

```bash
make install PROVIDER_VERSION=1.0.0
# or
make install MIRROR_DIR=/path/to/your/plugins/registry.terraform.io/mistedo-cloud/mistedo/0.0.1/darwin_arm64
```

4. Run **`terraform init`** and **`terraform plan`** in your project. Terraform will load the provider from the filesystem mirror and will not contact the registry.

### If you see `Invalid provider registry host`

Terraform is trying to use the public registry for `registry.terraform.io/mistedo-cloud/mistedo`, which is not published there. That usually means **no provider installation override is active**.

Checklist:

- **`~/.terraformrc` exists** (or `%APPDATA%\terraform.rc` on Windows) and contains the `filesystem_mirror` + `direct { exclude = [...] }` block from [`terraform.rc.example`](terraform.rc.example). Replace **`YOUR_HOME`** with a real absolute path; a literal `YOUR_HOME` breaks the mirror path.
- **No conflicting `TF_CLI_CONFIG_FILE`** pointing at another file without the mirror block.
- After changing the mirror path or version, run **`make install`** again so the binary exists under `~/.terraform.d/plugins/registry.terraform.io/mistedo-cloud/mistedo/<version>/<os>_<arch>/`.

### Alternative: `dev_overrides` (no mirror layout)

For frequent rebuilds, point Terraform at a directory that contains `terraform-provider-mistedo`. See [`terraform.dev-overrides.rc.example`](terraform.dev-overrides.rc.example) and set `TF_CLI_CONFIG_FILE` to your edited copy.

**Caveat (Terraform 1.14.x):** `terraform init` can still contact the registry for provider metadata and fail with the same “Invalid provider registry host” error when the provider is unpublished. `terraform plan` / `apply` then use your local binary. For a workflow where **`terraform init` must succeed**, use the **filesystem mirror** above instead of `dev_overrides`.

## Development

```bash
go test ./...
```

## License

See [LICENSE](LICENSE) file.
