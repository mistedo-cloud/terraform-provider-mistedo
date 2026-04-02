---
page_title: "mistedo_instance_group Resource - terraform-provider-mistedo"
subcategory: "Compute"
description: |-
  Orders a catalog service (instance group): multiple VMs from one template via ManageIQ cart API.
---

# mistedo_instance_group (Resource)

Creates a **service** in the regional compute API by posting to **`POST /api/compute/v1/service_orders/cart/service_requests`** with `action: add`. The provider waits until **`lifecycle_state`** is **`provisioned`**, the expected number of VMs exist, and each VM has an IPv4 on a **`nic`** allocation, then loads **per-VM disks** from **`GET /vms/{id}?attributes=disks`**.

**In-place API updates** (changing VM count, disks, template, subnet, etc.) are **not** supported. Use `terraform apply -replace` on this resource when you need a new service. **`terraform apply`** may still run an in-place **refresh** when only **computed** attributes change (for example `instances`, `lifecycle_state`) so state matches the API.

---

## Subnet, CIDR, and IP

- **`subnet`** is the **subnet name** from the API (`cloud_subnets.name`). It is usually the **canonical** network name, e.g. `<location>_<account>_<short_name>`, not necessarily the short `name` you set on `mistedo_network`. Wire **`mistedo_network.<n>.canonical_name`** into **`subnet`** when both resources are in the same stack.
- **`private_ipv4_cidr`** is **always computed** from that subnet in the API. You **cannot** set it in Terraform.
- **`private_ip_allocation`**: `auto` (platform picks an address) or `manual`. With **`manual`**, set **`private_ipv4_address`** to an IPv4 **inside** the subnet CIDR.
- **`region_number`** in the order payload is **computed** from the **third octet** of the subnet’s IPv4 CIDR (platform contract used in the live API).

**Vlan** sent to the API is `"<subnet> (<subnet>)"`.

---

## Disks

- **`boot_disk`** (required): system disk for every VM.
- **`data_disk`** (optional): at most **one** extra disk at order time. Additional data disks can appear later (reconfiguration); they show up under **`instances[].data_disks`** on refresh.

---

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
  # password: omit to auto-generate, or set var.admin_password (do not use "")

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

---

## Arguments

| Name | Description |
|------|-------------|
| `name` | Service name. |
| `description` | Optional description. |
| `template_id` | Catalog template id (`mistedo_instance_template`). |
| `desired_instance_count` | Number of VMs (1–500). |
| `cpu_cores` | vCPU per VM. |
| `memory_mib` | RAM per VM (MiB). |
| `boot_disk` | Required nested object: `boot_disk = { size_gib, type }` (not a `boot_disk { }` block). |
| `data_disk` | Optional nested object: `data_disk = { size_gib, type }`. |
| `subnet` | Subnet **name** in the API (often `mistedo_network.<n>.canonical_name`). |
| `private_ip_allocation` | `auto` or `manual`. |
| `private_ipv4_address` | Required for `manual`; must lie in computed CIDR. |
| `security_group_ids` | Up to one id today (first element sent to API). |
| `pass_auth` | API `pass_auth` string. |
| `password` | Optional, computed, sensitive. **Omit** to auto-generate at create. Do not set to `""` (breaks plan validation for sensitive values). |
| `ssh_public_keys` | Optional list. |
| `public_remote_access` | Optional list (e.g. `22/tcp`). |
| `managed_access` | Optional. |
| `user_data` | Optional cloud-init. |

## Attributes (computed)

| Name | Description |
|------|-------------|
| `id` | Service id. |
| `private_ipv4_cidr` | CIDR from the resolved subnet. |
| `lifecycle_state` | e.g. `provisioned`. |
| `region_number` | Derived for the order. |
| `vlan` | Vlan string sent to the API. |
| `instances` | Per VM: `boot_disk`, `data_disks[]`, `network_interfaces[]`. |
