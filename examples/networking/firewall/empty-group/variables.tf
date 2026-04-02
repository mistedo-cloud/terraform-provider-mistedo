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

variable "group_name" {
  type        = string
  description = "Short security group name (API may prefix it)."
  default     = "tf_example_empty_sg"
}
