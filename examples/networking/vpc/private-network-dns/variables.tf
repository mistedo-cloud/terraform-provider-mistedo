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
  type    = string
  default = "tf_example_net_dns"
}

variable "network_cidr" {
  type    = string
  default = "10.251.0.0/24"
}

variable "dns_nameservers" {
  type        = list(string)
  description = "Recursive resolvers passed to the subnet (optional but set here to demonstrate the argument)."
  default     = ["1.1.1.1", "8.8.8.8"]
}
