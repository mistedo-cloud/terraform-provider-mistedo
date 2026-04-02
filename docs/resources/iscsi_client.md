---
page_title: "mistedo_iscsi_client Resource - Mistedo Terraform Provider"
subcategory: "Storage"
description: |-
  iSCSI initiator client (CHAP and optional disk attachments) via Storage API v2.
---

# mistedo_iscsi_client (Resource)

Registers an **iSCSI client** by **IQN** with **CHAP** credentials. You can attach **disks** in the same apply (second API call) or update attachments later: **`disks`** changes use POST assign / DELETE unassign; **name**, **owner**, **iqn**, and CHAP fields use PUT.

## Example

```hcl
resource "mistedo_iscsi_client" "db" {
  name          = "database-server"
  owner         = "ops@example.com"
  iqn           = "iqn.1994-05.com.redhat:abc123"
  chap_username = "database"
  chap_password = var.chap_password

  disks = [
    {
      id   = tonumber(mistedo_iscsi_disk.data.id)
      name = mistedo_iscsi_disk.data.name
    }
  ]
}
```

**`chap_password`**: 12–16 characters; only **letters, digits**, and **`. : @ _ - /`**.

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `name` | Yes | Human-readable name. |
| `owner` | Yes | Owner email. |
| `iqn` | Yes | Initiator IQN. |
| `chap_username` | Yes | CHAP username. |
| `chap_password` | Yes | CHAP password (**sensitive**); length and charset per API rules above. |
| `disks` | No | List of `{ id, name }` for the assign payload. Omit for a client with no disks yet. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Numeric client id from the Storage API (string). |

## Import

```shell
terraform import mistedo_iscsi_client.db <client_id>
```

After import, set **`chap_password`** in configuration if the API does not return it, then run **apply** to align state.

## Notes

- Create [`mistedo_iscsi_disk`](iscsi_disk.md) resources before referencing them in **`disks`**.
