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
  default = "tf-example-http"
}

variable "hostname" {
  type        = string
  description = "Public hostname for the route (must be allowed on your ALB/DNS)."
  default     = "tf-example-http.example.com"
}

variable "cloud_gateway_id" {
  type        = number
  description = "From data.mistedo_load_balancers (gateway id)."
}

variable "backend_service_id" {
  type        = number
  description = "From data.mistedo_lb_backend_services (service id)."
}
