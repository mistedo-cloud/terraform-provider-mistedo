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
  description = "Short name for mistedo_network (API also uses it as subnet.name)."
  default     = "tf_example_net"
}

variable "network_cidr" {
  type        = string
  description = "RFC1918 CIDR that does not overlap existing tenant networks."
  default     = "10.250.0.0/24"
}
