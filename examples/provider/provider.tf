terraform {
  required_providers {
    truenas = {
      source  = "truenas/truenas"
      version = "~> 0.1"
    }
  }
}

provider "truenas" {
  endpoint = "wss://truenas.example.com/api/current"
  api_key  = var.truenas_api_key
  insecure = true
}

variable "truenas_api_key" {
  description = "TrueNAS API key"
  sensitive   = true
}
