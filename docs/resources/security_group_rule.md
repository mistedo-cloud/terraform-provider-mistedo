---
page_title: "mistedo_security_group_rule Resource - Mistedo Terraform Provider"
subcategory: "Compute"
description: |-
  One firewall rule (ingress or egress) inside a mistedo_security_group: ports, protocol, CIDR or remote group.
---

# mistedo_security_group_rule (Resource)

Adds **one rule** to an existing [`mistedo_security_group`](security_group.md), for example “allow TCP 22 from this CIDR” or “allow egress for IPv4”.

Changing **any** tracked argument forces **replacement** (delete rule, create again). There is no silent in-place edit.

The provider **loads the parent group**; set `security_group_id = mistedo_security_group.<name>.id`. Several rules in one `apply` are **serialized per security group** so parallel creates cannot attach the wrong rule identity.

## Example

### SSH from one IPv4 network

```hcl
resource "mistedo_security_group" "app" {
  name = "myapp_fw"
}

resource "mistedo_security_group_rule" "ssh" {
  security_group_id = mistedo_security_group.app.id

  direction        = "ingress"
  protocol         = "tcp"
  port_range       = "22"
  network_protocol = "IPV4"
  remote_ip_subnet = "203.0.113.0/24"
}
```

### Port range

```hcl
resource "mistedo_security_group_rule" "app_ports" {
  security_group_id = mistedo_security_group.app.id

  direction        = "ingress"
  protocol         = "tcp"
  port_range       = "8080-8090"
  network_protocol = "IPV4"
  remote_ip_subnet = "0.0.0.0/0"
}
```

- `port_range` — one port (`22`) or `min-max` (`8080-8090`).
- `network_protocol` — often **`IPV4`** or **`IPV6`** for the compute API.
- Omit `remote_ip_subnet` when the rule targets another group via `remote_group_id`.

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `security_group_id` | Yes | `mistedo_security_group.<name>.id`. Changing it replaces the rule. |
| `direction` | Yes | `ingress` (to instances) or `egress` (from instances). |
| `port_range` | No | Single port or `min-max` string. |
| `protocol` | No | e.g. `tcp`, `udp`, `icmp`; can be empty for some wide egress rules. |
| `network_protocol` | No | Often `IPV4` / `IPV6` when the API expects it. |
| `remote_group_id` | No | Another security group for group-to-group rules. |
| `remote_ip_subnet` | No | Source CIDR (API `source_ip_range`), e.g. `192.0.2.0/24`. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | `<security_group_id>/<rule_ems_ref>` (UUID part is **lowercased** in state). |
| `ems_ref` | Value used for **delete** (normalized like `id`). |

## Import

Format: **`<group_cloud_id>/<rule_ems_ref>`** (quote the id in the shell):

```shell
terraform import mistedo_security_group_rule.ssh '81000000000108/b99f8de8-bed3-4e93-8574-d2349a4f1d57'
```

## Notes

- Create the **security group** before rules; rules **depend** on it.
- Duplicating a rule the platform treats as default may return **“Rule already exists”** — use a distinct rule or rely on the group’s cleaned defaults after [`mistedo_security_group`](security_group.md) create.
- The provider **merges read results** with your configuration for optional fields when GET omits values sent on create, to avoid spurious **replacement**.
- Omitted optional arguments are **`null`** in state; the API may use **`""`**. The provider maps empty API strings back to **`null`** when you did not set the argument, so **RequiresReplace** does not fire incorrectly.
