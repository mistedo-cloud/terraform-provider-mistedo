---
page_title: "mistedo_instance_group Resource - Mistedo Terraform Provider"
subcategory: "Compute"
description: |-
  Orders a catalog service (instance group): multiple VMs from one template via the compute cart API.
---

# mistedo_instance_group (Resource)

Creates a **service** by posting to **`POST /api/compute/v1/service_orders/cart/service_requests`** with `action: add`. The provider waits until **`lifecycle_state`** is **`provisioned`**, the expected VM count exists, and each VM has an IPv4 on a **`nic`** allocation, then loads **per-VM disks** from **`GET /vms/{id}?attributes=disks`**.

**In-place API updates** (VM count, disks, template, subnet, etc.) are **not** supported. Use `terraform apply -replace` when you need a new service. **`apply`** may still **refresh** when only **computed** attributes change (`instances`, `lifecycle_state`, …).

## Example

```hcl
resource "mistedo_network" "app" {
  name = "my_net"
  cidr = "10.0.0.0/24"
}

data "mistedo_instance_template" "fedora" {
  name    = "Fedora Cloud"
  version = "39-240223"
}

resource "mistedo_instance_group" "app" {
  name        = "my-service"
  description = "Demo"

  template_id            = data.mistedo_instance_template.fedora.id
  desired_instance_count = 1

  cpu_cores  = 1
  memory_mib = 1024

  boot_disk = {
    size_gib = 10
    type     = "nvme"
  }

  data_disk = {
    size_gib = 10
    type     = "nvme"
  }

  subnet = mistedo_network.app.canonical_name

  private_ip_allocation = "auto"

  security_group_ids = ["81000000000108"]

  pass_auth = "disable_password"

  ssh_public_keys      = [var.ssh_public_key]
  public_remote_access = ["22/tcp"]
  managed_access       = "enable"

  user_data = <<-EOT
    #cloud-config
    runcmd:
      - touch /root/tf.txt
  EOT
}

output "vm_ips" {
  value = [for i in mistedo_instance_group.app.instances : i.network_interfaces]
}
```

Omit **`password`** to auto-generate at create, or set `var.admin_password`. Do **not** set `password = ""` (breaks sensitive plan validation).

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | Yes | Service name. |
| `description` | No | Description. |
| `template_id` | Yes | Catalog template id ([`mistedo_instance_template`](../data-sources/instance_template.md)). |
| `desired_instance_count` | Yes | Number of VMs (1–500). |
| `cpu_cores` | Yes | vCPU per VM. |
| `memory_mib` | Yes | RAM per VM (MiB). |
| `boot_disk` | Yes | Nested object: `boot_disk = { size_gib, type }` (not a `boot_disk { }` block). |
| `data_disk` | No | Nested object: `data_disk = { size_gib, type }`. At most **one** extra disk at order time. |
| `subnet` | Yes | Subnet **name** in the API (often `mistedo_network.<n>.canonical_name`). |
| `private_ip_allocation` | Yes | `auto` or `manual`. |
| `private_ipv4_address` | No | Required for `manual`; must lie inside the subnet CIDR. |
| `security_group_ids` | No | Up to one id today (first element sent to API). |
| `pass_auth` | Yes | API `pass_auth` string. |
| `password` | No | Optional, computed, sensitive. **Omit** to auto-generate. Do not set to `""`. |
| `ssh_public_keys` | No | Optional list. |
| `public_remote_access` | No | e.g. `22/tcp`. |
| `managed_access` | No | Optional. |
| `user_data` | No | Optional cloud-init. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Service id. |
| `private_ipv4_cidr` | CIDR from the resolved subnet (**always computed**; you cannot set it in Terraform). |
| `lifecycle_state` | e.g. `provisioned`. |
| `region_number` | Derived for the order (from subnet CIDR third octet in the live API contract). |
| `vlan` | Vlan string sent to the API (`"<subnet> (<subnet>)"`). |
| `instances` | Per VM: `boot_disk`, `data_disks[]`, `network_interfaces[]`. |

## Notes

### Subnet, CIDR, and IP

- **`subnet`** is the **subnet name** from the API (`cloud_subnets.name`), often the **canonical** network name like `<location>_<account>_<short_name>`. Wire **`mistedo_network.<n>.canonical_name`** into **`subnet`** when both resources share a stack.
- **`private_ipv4_cidr`** is **always computed** from the subnet; you cannot set it.
- **`private_ip_allocation`**: `auto` or `manual`. With **`manual`**, set **`private_ipv4_address`** inside the subnet CIDR.

### Disks

- **`boot_disk`** (required): system disk for every VM.
- **`data_disk`** (optional): one extra disk at order time; more may appear after reconfiguration under **`instances[].data_disk`**.

### Import

Import is not documented for this resource; manage lifecycle through Terraform create/destroy or `terraform apply -replace`.
