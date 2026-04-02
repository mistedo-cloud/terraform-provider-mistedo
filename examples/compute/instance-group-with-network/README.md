# Compute — instance group with network and firewall

End-to-end sample:

1. **`mistedo_network`** — tenant private network + subnet.  
2. **`mistedo_security_group`** + **`mistedo_security_group_rule`** — SSH ingress from **`ssh_ingress_cidr`**.  
3. **`data.mistedo_instance_template`** — catalog template id.  
4. **`mistedo_instance_group`** — order one VM from that template on the subnet, attached to the group.

**Important**

- **`subnet`** on the instance group must be the **API subnet name**. After create, that is usually **`mistedo_network.<name>.canonical_name`** (e.g. `dev_ha001_<short_name>`), not the short **`name`** you typed in **`mistedo_network`**.  
- **`boot_disk`** is a **nested object**: `boot_disk = { size_gib = …, type = … }`, not a `boot_disk { }` block.  
- **Password:** omit **`password`** to auto-generate. Do **not** set **`password = ""`** (breaks Terraform sensitive plan validation).  
- **Destroy order:** retire the instance group before the network. Terraform dependency edges normally do that; network delete may still **wait** until VM NICs disappear (provider retries and polls subnet **`network_ports`** when the API supports it).

**Resources:** `mistedo_network`, `mistedo_security_group`, `mistedo_security_group_rule`, `mistedo_instance_group`  
**Data source:** `mistedo_instance_template`
