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

variable "admin_cidr" {
  type        = string
  description = "CIDR allowed to SSH and reach HTTPS on the example rules."
  default     = "203.0.113.0/24"
}
