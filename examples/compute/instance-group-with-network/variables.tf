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

variable "network_name" {
  type        = string
  description = "Short name for mistedo_network (API subnet name is often prefixed; use canonical_name for orders)."
  default     = "tf_example_ig_net"
}

variable "network_cidr" {
  type        = string
  description = "Non-overlapping tenant CIDR for the single subnet."
  default     = "10.240.0.0/24"
}

variable "security_group_name" {
  type    = string
  default = "tf_example_ig_fw"
}

variable "ssh_ingress_cidr" {
  type        = string
  description = "Source CIDR for SSH ingress on the example security group."
  default     = "203.0.113.0/24"
}

variable "template_name" {
  type    = string
  default = "Fedora Cloud"
}

variable "template_version" {
  type        = string
  description = "Exact version segment; empty = name-only lookup (must be unique)."
  default     = ""
}

variable "instance_group_name" {
  type    = string
  default = "tf-example-ig"
}
