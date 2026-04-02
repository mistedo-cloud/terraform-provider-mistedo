---
page_title: "mistedo_lb_certificate Resource - Mistedo Terraform Provider"
subcategory: "ALB"
description: |-
  Uploads a TLS certificate for HTTPS on the Mistedo application load balancer.
---

# mistedo_lb_certificate (Resource)

Uploads a **TLS certificate** and private key so [`mistedo_lb_route`](lb_route.md) can use **HTTPS** via `certificate_id` or related TLS settings.

**PEM material is sensitive** — it is stored in **Terraform state**. Protect state (remote backend, encryption, access control).

The API rejects **duplicate certificate names** (HTTP **422**).

## Example

```hcl
resource "mistedo_lb_certificate" "app" {
  name            = "app.example.com"
  certificate_pem = file("${path.module}/cert.pem")
  private_key_pem = file("${path.module}/key.pem")
}
```

Optional intermediate chain:

```hcl
resource "mistedo_lb_certificate" "app_with_chain" {
  name            = "app.example.com"
  certificate_pem = file("${path.module}/cert.pem")
  private_key_pem = file("${path.module}/key.pem")
  ca_pem          = file("${path.module}/chain.pem")
}
```

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | Yes | Certificate name (often the primary **FQDN**). Changing **name** replaces the resource. |
| `certificate_pem` | Yes | Server certificate in PEM form (**sensitive**). |
| `private_key_pem` | Yes | Private key in PEM form (**sensitive**). |
| `ca_pem` | No | Intermediate / CA chain (**sensitive**). |
| `dest_ca_pem` | No | For re-encrypt scenarios when required (**sensitive**). |
| `owner` | No | Owner email; API may default from account context. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Certificate id in the API (number). |
| `account` | Account identifier. |
| `cert_path` | Path as returned by the API. |
| `key_path` | Path as returned by the API. |
| `ca_path` | Path as returned by the API. |
| `dest_ca_path` | Path as returned by the API. |
| `created_at` | Timestamp when returned. |
| `updated_at` | Timestamp when returned. |

## Import

```shell
terraform import mistedo_lb_certificate.app 1
```

After import, run **`terraform plan`** — PEM fields are filled from API read where supported.

## Notes

- Provider default HTTP timeouts apply; there is no `timeouts` block on this resource.
