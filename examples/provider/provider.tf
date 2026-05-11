terraform {
  required_providers {
    opusdns = {
      source  = "opusdns/opusdns"
      version = "~> 1.0"
    }
  }
}

provider "opusdns" {
  # API key can also be set via OPUSDNS_API_KEY environment variable.
  api_key = "opk_your_api_key_here"
}
