---
page_title: "mistedo_vpc_route Resource - Mistedo Terraform Provider"
subcategory: "Compute"
description: |-
  Manages a static VPC route on the tenant default network router; router id is resolved by the provider.
---

# mistedo_vpc_route (Resource)

Each account has a **default network router** you cannot create or delete in Terraform. This resource **adds and removes static routes** on that router.

You set **`destination`** (CIDR) and **`nexthop`**. The provider calls **`GET .../network_routers`**, resolves the default router, then **`POST .../network_routers/{id}`** with `action: add_route` or `remove_route`. If the API returns a `task_id`, the provider waits for the task like other ManageIQ actions.

## Example

```hcl
resource "mistedo_vpc_route" "to_peer_net" {
  destination = "10.220.0.0/16"
  nexthop     = "10.220.0.2"
}
```

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `destination` | Yes | Destination CIDR. Changing it **replaces** the resource (remove old route, add new). |
| `nexthop` | Yes | Next hop IP. Changing it **replaces** the resource. |

## Attributes

| Name | Description |
|------|-------------|
| `id` | Surrogate id: `router_id|destination|nexthop`. |
| `router_id` | Resolved cloud id of the default network router (computed). |

## Import

Import id is **`destination|nexthop`** (single `|` — CIDRs use `/`, not `|`):

```shell
terraform import mistedo_vpc_route.to_peer_net '10.220.0.0/16|10.220.0.2'
```

The provider resolves the default router and checks the route exists before writing state.

## Notes

### Default router selection

1. If the API returns **exactly one** router, that id is used.
2. Otherwise the provider picks the router whose **`name`** equals **`<location>_<account>_default`** (e.g. `dev_ha001_default`), using provider **`location`** and **`account`**.
3. If several routers exist and none match, Terraform returns an error (there is no `router_id` argument).

### Surrogate `id`

Routes in **`extra_attributes.routes`** are only **`destination` + `nexthop`** — there is no separate route id. The provider uses:

`id = "<router_id>|<destination>|<nexthop>"`

**Read** reloads the router and checks the pair still exists; otherwise the resource is removed from state (drift or manual delete).

### Other

- **`router_id`** is never set in configuration; it is always **computed** after resolution.
- Normal IPv6 addresses use `:` and do not contain `|`; import remains unambiguous.
