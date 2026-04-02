# Compute — instance template (data source only)

Uses **`data.mistedo_instance_template`** to resolve a **catalog VM template** by **`name`** and optional **`version`**. The API match is **case-sensitive**.

Outputs give **`template_id`** (use as **`mistedo_instance_group.template_id`**), **`full_name`**, and **`guid`** when present.

For a full order (network + security group + instance group), see **`../instance-group-with-network/`**.

**Data source:** `mistedo_instance_template`
