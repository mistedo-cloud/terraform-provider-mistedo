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

variable "cert_name" {
  type        = string
  description = "Certificate name in ALB (usually the primary hostname)."
  default     = "tf-example-cert.example.com"
}
