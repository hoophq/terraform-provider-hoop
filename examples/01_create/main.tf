terraform {
  required_providers {
    hoop = {
      source  = "registry.terraform.io/local/hoop"
      version = "1.0.0"
    }
  }
}

provider "hoop" {
  api_key = var.hoop_api_key
  api_url = var.api_url
}
