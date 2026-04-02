# Compute — security group + ingress rules

Creates **`mistedo_security_group`** and two **`mistedo_security_group_rule`** resources (**SSH** and **HTTPS**) scoped to **`admin_cidr`** (defaults to documentation prefix **203.0.113.0/24**).

Shows **optional** rule arguments most teams set: **`protocol`**, **`port_range`**, **`network_protocol`**, **`remote_ip_subnet`**.
