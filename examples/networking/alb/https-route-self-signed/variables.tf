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

variable "route_name" {
  type    = string
  default = "tf-example-https"
}

variable "hostname" {
  type        = string
  description = "Must match the certificate CN / SAN (self-signed below uses this)."
  default     = "tf-example-https.example.com"
}

variable "cloud_gateway_id" {
  type = number
}

variable "backend_service_id" {
  type = number
}

variable "healthcheck_path" {
  type        = string
  description = "Optional healthcheck path (demonstrates the healthcheck block)."
  default     = "/healthz"
}
