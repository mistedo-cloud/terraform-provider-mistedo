---
page_title: "mistedo_network Resource - Mistedo Terraform Provider"
subcategory: "Compute"
description: |-
  Creates and deletes a tenant private network (VPC-style) with one subnet; uses the regional compute API and waits for async tasks.
---

# mistedo_network (Resource)

A **private network** is an isolated space for your VMs. You set a single **`name`** in Terraform; it is sent as the **network name** and as **`subnet.name`** in the create API. You also set **subnet CIDR** and optional **DNS servers**. The API uses **IPv4** with fixed **`network_protocol`** / **`ip_version`** for that subnet; those are **not** configurable in Terraform.

Create and delete are **asynchronous**. The provider waits for the ManageIQ task and polls until the subnet exists (up to about **15 minutes** per operation).

**Destroy** can fail if **VM network interfaces** still use the subnet. The client **waits** until subnet **`network_ports`** are empty when the API returns them, and **retries** transient errors (e.g. **500**) for up to about **20 minutes**; the Terraform delete timeout is **30 minutes**.

The API may **rewrite the name** (e.g. `<location>_<account>_` prefix). Terraform keeps your configured `name` in state; the **`canonical_name`** attribute holds the full API name.

## Example

```hcl
resource "mistedo_network" "app" {
  name = "myapp_net"
  cidr = "10.120.0.0/24"

  dns_nameservers = ["213.222.50.226"]
}

output "network_id" {
  value = mistedo_network.app.id
}

output "subnet_id" {
  value = mistedo_network.app.subnet_id
}
```

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | Yes | Label for both the network and its subnet on create. The platform may store a longer name; see `canonical_name`. Changing it **replaces** the network. |
| `cidr` | Yes | CIDR of the **single** subnet (e.g. `10.0.0.0/24`). Changing it **replaces** the network. |
| `dns_nameservers` | No | DNS servers for the subnet. If **omitted**, Terraform leaves the attribute **unset in state** even if the platform fills defaults later (avoids drift). Set it when you want Terraform to **manage** the list. Changing it **replaces** the network. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Cloud **network** id from the API. |
| `canonical_name` | Full network name returned by the API. |
| `ems_ref` | ManageIQ reference for the network. |
| `status` | Status string when the API provides it. |
| `mtu` | MTU from `extra_attributes` when exposed. |
| `subnet_id` | Cloud id of the first (only) subnet. |
| `subnet_ems_ref` | ManageIQ reference for the subnet when present. |
| `gateway` | Subnet default gateway when the API returns it. |

## Import

Use the numeric **cloud network id** from the UI or API:

```shell
terraform import mistedo_network.app 81000000000123
```

After import, run **`terraform plan`**. You may need to align `name`, `cidr`, and optional fields with what the platform returns.

## Notes

- There is **no generic in-place update** for this resource; changing replaceable fields triggers **destroy + create**.
- Requests use **`x-miq-group`** and **`x-auth-*`** headers for `/api/compute/v1/...`.
- Fix [provider](../index.md) **auth** (e.g. `auth_url` for dev) before debugging API errors.
