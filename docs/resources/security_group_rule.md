---
page_title: "mistedo_security_group_rule Resource - terraform-provider-mistedo"
subcategory: "Compute"
description: |-
  One firewall rule (ingress or egress) inside a mistedo_security_group: ports, protocol, CIDR or remote group.
---

# mistedo_security_group_rule (Resource)

Adds **one rule** to an existing [`mistedo_security_group`](security_group.md): for example “allow TCP 22 from this CIDR” or “allow egress for IPv4”.

Changing **any** argument Terraform tracks forces **replacement** (delete rule + create again). There is no silent in-place edit.

The cloud API needs technical fields (`ems_ref` of the parent group). The provider **loads the group for you**; you only pass `security_group_id = mistedo_security_group.<name>.id`.

Several rules in one `apply` are supported: the client **serializes** rule creation per security group and matches the new rule by id and by the attributes you requested, so parallel creates cannot attach the wrong `ems_ref` to a resource.

---

## Examples

### Allow SSH from one IPv4 network

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

### Port range and optional fields

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

* `port_range` — one port (`22`) or `min-max` (`8080-8090`).
* `network_protocol` — often **`IPV4`** or **`IPV6`** for the compute API (match what your project expects).
* Leave `remote_ip_subnet` empty if the rule targets another security group via `remote_group_id` instead.

---

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `security_group_id` | Yes | `mistedo_security_group.<name>.id`. Changing it replaces the rule. |
| `direction` | Yes | `ingress` (traffic **to** instances) or `egress` (traffic **from** instances). |
| `port_range` | No | Single port or `min-max` string. |
| `protocol` | No | e.g. `tcp`, `udp`, `icmp`; can be empty for some wide egress rules. |
| `network_protocol` | No | Often `IPV4` / `IPV6` when the API expects it. |
| `remote_group_id` | No | Reference to another security group when the rule is group-to-group. |
| `remote_ip_subnet` | No | Source CIDR (maps to API `source_ip_range`), e.g. `192.0.2.0/24`. |

---

## Attributes

* `id` — Terraform id: **`<security_group_id>/<rule_ems_ref>`** (slash-separated). The UUID part is **lowercased** in state so `terraform plan` stays clean if the API returns the same UUID with different letter case.
* `ems_ref` — Value used for **delete** (stored lowercase; the remove call uses this normalized form).

---

## Import

Format: **`<group_cloud_id>/<rule_ems_ref>`** (both from the API or UI). Quote the id in the shell:

```shell
terraform import mistedo_security_group_rule.ssh '81000000000108/b99f8de8-bed3-4e93-8574-d2349a4f1d57'
```

---

## Practical notes

* Create the **security group first**; rules **depend** on it.
* Duplicating a “wide open” egress rule that the platform treats as default may return **“Rule already exists”** — use a distinct rule or rely on the group’s cleaned defaults after [`mistedo_security_group`](security_group.md) create.
* The provider **merges read results with your configuration** for optional fields when the GET API omits values that were sent on create — otherwise a later `terraform plan` could falsely request **replacement** (same symptom as drift on `id` / `ems_ref`).
* Omitted optional arguments are **`null`** in state; the API often encodes “empty” as **`""`**. Those differ in Terraform and, with **RequiresReplace**, force replacement — the provider maps empty API strings back to **`null`** when you did not set the argument.
