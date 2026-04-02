terraform {
  required_version = ">= 1.3.0"
  required_providers {
    mistedo = {
      source  = "mistedo-cloud/mistedo"
      version = ">= 0.0.1"
    }
    tls = {
      source  = "hashicorp/tls"
      version = ">= 4.0.0"
    }
  }
}
