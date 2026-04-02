# ALB — upload certificate (required fields only)

**`mistedo_lb_certificate`** with **`name`**, **`certificate_pem`**, **`private_key_pem`** only.  
PEMs come from the **`tls`** provider so nothing secret is committed.

To add an **optional** chain, set **`ca_pem`** (and optionally **`dest_ca_pem`**) on the same resource.
