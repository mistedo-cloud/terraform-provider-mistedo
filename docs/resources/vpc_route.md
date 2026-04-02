---
page_title: "mistedo_vpc_route Resource - terraform-provider-mistedo"
subcategory: "Compute"
description: |-
  Manages a static VPC route on the tenant default network router (add_route / remove_route); router id is resolved by the provider.
---

# mistedo_vpc_route (Resource)

Each cloud account has a **default network router** you **cannot** create or delete in Terraform. You can **add and remove static VPC routes** on that router with this resource.

You only configure **`destination`** (CIDR) and **`nexthop`**. The provider calls **`GET .../network_routers`**, resolves the default router (see below), then **`POST .../network_routers/{id}`** with `action: add_route` or `remove_route`. Calls are **synchronous** in normal operation; if the API ever returns a `task_id`, the provider still waits for the task like other ManageIQ actions.

---

## How the default router is chosen

1. If the API returns **exactly one** router, that id is used.
2. Otherwise the provider picks the router whose **`name`** equals **`<location>_<account>_default`** (same pattern as the platform, e.g. `dev_ha001_default`), using your provider **`location`** and **`account`**.

If several routers exist and none match that name, Terraform returns an error (there is no `router_id` argument to set).

---

## Terraform `id` (no API route id)

Routes in **`extra_attributes.routes`** are only **`destination` + `nexthop`** strings — there is no separate route id. The provider therefore uses a **surrogate id**:

`id = "<router_id>|<destination>|<nexthop>"`

**Read** reloads the router and checks that this pair still exists; if not, the resource is removed from state (drift or manual delete).

---

## Example

```hcl
resource "mistedo_vpc_route" "to_peer_net" {
  destination = "10.220.0.0/16"
  nexthop     = "10.220.0.2"
}
```

---

## Arguments

| Name | Required | Description |
|------|----------|-------------|
| `destination` | yes | Destination CIDR for the route. Changing it **replaces** the resource (remove old + add new). |
| `nexthop` | yes | Next hop IP. Changing it **replaces** the resource. |

---

## Attributes

| Name | Description |
|------|-------------|
| `id` | Surrogate id: `router_id|destination|nexthop`. |
| `router_id` | Resolved cloud id of the default network router (computed only). |

---

## Import

Import id is **`destination|nexthop`** (one `|` separator — CIDRs use `/`, not `|`):

```shell
terraform import mistedo_vpc_route.to_peer_net '10.220.0.0/16|10.220.0.2'
```

The provider resolves the default router and checks the route exists before writing state.

---

## Practical notes

* **No `router_id` in configuration** — it is always **computed** after resolution.
* **IPv6**: if `destination` or `nexthop` ever contain `|`, import would break; normal IPv6 addresses use `:` and are fine.
