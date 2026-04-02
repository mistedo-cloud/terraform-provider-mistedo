---
page_title: "mistedo_network Resource - terraform-provider-mistedo"
subcategory: "Compute"
description: |-
  Creates and deletes a tenant private network (VPC-style) with one subnet on Mistedo; uses the regional compute API and waits for async tasks.
---

# mistedo_network (Resource)

A **private network** is an isolated **Layer-2/Layer-3 space** for your VMs. You set a single **`name`** in Terraform; the same string is sent as the **network name** and as **`subnet.name`** in the create API. You also set **subnet CIDR** and optional **DNS servers**. The API uses **IPv4** with **`network_protocol: ipv4`** and **`ip_version: 4`** for that subnet; those are **not** configurable in Terraform.

Create and delete are **asynchronous** on the server. The provider **waits for the ManageIQ task** and then **polls until the subnet appears**, similar to other compute resources (up to about **15 minutes** per operation).

**Destroy** can fail if **VM network interfaces** still use the subnet (for example right after an instance group is retired). The client then **waits** until subnet **`network_ports`** are empty when the API returns them, and **retries** transient HTTP errors (for example **500**) for up to about **20 minutes**; the Terraform delete timeout is **30 minutes**.

The API often **rewrites the name** (for example it adds a `<location>_<account>_` prefix). Terraform keeps your configured `name` in state so the next plan stays clean; the full name from the API is in **`canonical_name`**.

---

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

---

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | yes | One label for both the network and its subnet in the API (same `name` and `subnet.name` on create). The platform may store a longer network name; see `canonical_name`. Changing it **replaces** the network. |
| `cidr` | yes | CIDR for the **single** subnet created with the network (for example `10.0.0.0/24`). Changing it **replaces** the network. |
| `dns_nameservers` | no | List of DNS server addresses for the subnet. If you **omit** this argument entirely, Terraform leaves it **unset in state** even if the platform fills in servers later (avoids endless drift). Set it when you want Terraform to **track** the list. Changing it **replaces** the network. |

---

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

---

## Import

Import uses the numeric **cloud network id** from the UI or API:

```shell
terraform import mistedo_network.app 81000000000123
```

After import, run **`terraform plan`**. You may need to align `name`, `cidr`, and optional fields with what the platform returns.

---

## Practical notes

* **No in-place update:** there is no generic “update network” in this resource; changing replaceable fields triggers **destroy + create**.
* **Same headers as security groups:** calls use **`DoManageIQ`** (`x-miq-group` and `x-auth-*`) for `/api/compute/v1/...`.
