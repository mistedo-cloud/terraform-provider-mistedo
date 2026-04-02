variable "mistedo_username" {
  type = string
}

variable "mistedo_password" {
  type      = string
  sensitive = true
}

variable "mistedo_account" {
  type = string
}

variable "mistedo_location" {
  type = string
}

variable "mistedo_role" {
  type = string
}

variable "peer_cidr" {
  type        = string
  description = "Remote VPC or on-prem prefix reachable via nexthop."
  default     = "10.220.0.0/16"
}

variable "peer_nexthop" {
  type        = string
  description = "Next hop inside your network (VPN/peer router)."
  default     = "10.220.0.2"
}
