---
page_title: "mistedo_iscsi_client Resource - terraform-provider-mistedo"
subcategory: "Storage"
description: |-
  iSCSI initiator client (CHAP + optional disk attachments) via Storage API v2.
---

# mistedo_iscsi_client (Resource)

Registers an **iSCSI client** (consumer) by **IQN** with required **CHAP** credentials. You can attach **disks** in the same apply (second API call) or manage attachments later: **`disks`** changes use POST assign / DELETE unassign; **name**, **owner**, **iqn**, and CHAP use PUT.

## Example

```hcl
resource "mistedo_iscsi_client" "db" {
  name           = "database-server"
  owner          = "ops@example.com"
  iqn            = "iqn.1994-05.com.redhat:abc123"
  chap_username  = "database"
  chap_password  = "MySecret12@ab" # 12–16 chars; allowed: letters, digits, . : @ _ - /

  disks = [
    {
      id   = tonumber(mistedo_iscsi_disk.data.id)
      name = mistedo_iscsi_disk.data.name
    }
  ]
}
```

## Arguments

- `name` (Required) — Human-readable name.
- `owner` (Required) — Owner email.
- `iqn` (Required) — Initiator IQN.
- `chap_username` (Required) — CHAP username.
- `chap_password` (Required, sensitive) — CHAP password: **12–16** characters; only **letters, digits**, and **`. : @ _ - /`**.
- `disks` (Optional) — List of `{ id, name }` matching the Storage API assign payload. Omit for a client with no disks yet; add or remove blocks later to sync attachments.

## Attributes

| Name | Description |
|------|-------------|
| `id` | Numeric client id from the Storage API (string). |

## Import

```bash
terraform import mistedo_iscsi_client.db <client_id>
```

After import, set **`chap_password`** in configuration if the API does not return it (then run apply to align state).
