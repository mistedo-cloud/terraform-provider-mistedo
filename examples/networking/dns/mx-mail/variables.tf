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

variable "zone_name" {
  type        = string
  description = "FQDN for mistedo_dns_zone (no trailing dot)."
}

variable "mail_host" {
  type        = string
  description = "Mail exchanger hostname for MX RDATA (e.g. mail.example.com)."
  default     = "mail.example.com"
}
