---
page_title: "mistedo_lb_certificate Resource - terraform-provider-mistedo"
subcategory: "ALB"
description: |-
  Uploads a TLS certificate for HTTPS on the Mistedo application load balancer.
---

# mistedo_lb_certificate (Resource)

Uploads a **TLS certificate** (and private key) so [`mistedo_lb_route`](lb_route.md) can use **HTTPS** with `certificate_id` or TLS settings the API expects.

**PEM files are sensitive** — they are stored in **Terraform state**. Protect state (remote backend, encryption, access control).

The API rejects **duplicate certificate names** (HTTP **422**).

---

## Example

Place `cert.pem` and `key.pem` next to the Terraform files (or use another path):

```hcl
resource "mistedo_lb_certificate" "app" {
  name              = "app.example.com"
  certificate_pem   = file("${path.module}/cert.pem")
  private_key_pem   = file("${path.module}/key.pem")
}
```

Optional chain:

```hcl
resource "mistedo_lb_certificate" "app" {
  name              = "app.example.com"
  certificate_pem   = file("${path.module}/cert.pem")
  private_key_pem   = file("${path.module}/key.pem")
  ca_pem            = file("${path.module}/chain.pem")
}
```

---

## Arguments

* `name` — (Required) Certificate name (often the primary **FQDN**). Changing **name** replaces the resource.
* `certificate_pem` — (Required, **Sensitive**) Server certificate in PEM form.
* `private_key_pem` — (Required, **Sensitive**) Private key in PEM form.
* `ca_pem` — (Optional, **Sensitive**) Intermediate / CA chain.
* `dest_ca_pem` — (Optional, **Sensitive**) For re-encrypt scenarios when required.
* `owner` — (Optional) Owner email; API may default from account context.

---

## Attributes

* `id` — Certificate id in the API (number).
* `account` — Account identifier.
* `cert_path`, `key_path`, `ca_path`, `dest_ca_path` — Paths as returned by the API.
* `created_at`, `updated_at` — Timestamps.

---

## Import

```shell
terraform import mistedo_lb_certificate.app 1
```

After import, run **`terraform plan`** — PEM fields are filled from a read of the API where supported.

---

## Timeouts

Provider default HTTP timeouts apply.
