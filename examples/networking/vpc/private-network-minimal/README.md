# Compute — private network (required only)

Creates **`mistedo_network`** with **`name`** and **`cidr`** only.  
Optional **`dns_nameservers`** is left out so Terraform does not track resolver IPs (avoids drift if the API injects defaults).

Choose a **non-overlapping CIDR** in your tenant before apply.
