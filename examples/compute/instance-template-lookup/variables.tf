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

variable "template_name" {
  type        = string
  description = "Catalog template product segment (before ':' in the full display name). Case-sensitive."
  default     = "Fedora Cloud"
}

variable "template_version" {
  type        = string
  description = "Version segment after ':' for an exact match. Leave empty to match by name only (must be unique)."
  default     = ""
}
