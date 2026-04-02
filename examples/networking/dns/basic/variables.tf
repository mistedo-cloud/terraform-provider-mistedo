variable "mistedo_username" {
  type        = string
  description = "Keycloak username (often an email)."
}

variable "mistedo_password" {
  type        = string
  sensitive   = true
  description = "Keycloak password."
}

variable "mistedo_account" {
  type        = string
  description = "Tenant account id (x-auth-account)."
}

variable "mistedo_location" {
  type        = string
  description = "Region code, e.g. dev → api.dev.mistedo.by."
}

variable "mistedo_role" {
  type        = string
  description = "API role, e.g. owner."
}

variable "zone_name" {
  type        = string
  description = "FQDN of a new hosted zone (no trailing dot), e.g. demo.example.com — must be a domain you are allowed to host."
}

variable "www_content" {
  type        = string
  description = "IPv4 for the www A record (documentation: TEST-NET-3)."
  default     = "203.0.113.10"
}
