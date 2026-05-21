terraform {
  required_providers {
    opusdns = {
      source  = "opusdns/opusdns"
      version = "~> 1.0"
    }
  }
}

# ---------------------------------------------------------------------------
# Input variables
#
# Every credential can be supplied either:
#   * Inline as a Terraform variable value (CLI -var, *.tfvars, etc.), OR
#   * Via the standard Terraform `TF_VAR_<name>` environment variables, e.g.
#       export TF_VAR_opusdns_api_key=opk_xxx
#
# As an additional convenience, the provider itself also reads native
# `OPUSDNS_*` environment variables (OPUSDNS_API_KEY, OPUSDNS_API_ENDPOINT)
# when the corresponding attribute is left null.
# ---------------------------------------------------------------------------

variable "opusdns_api_key" {
  description = "OpusDNS API key. Set via TF_VAR_opusdns_api_key."
  type        = string
  default     = null
  sensitive   = true
}

variable "opusdns_api_endpoint" {
  description = "Override the OpusDNS API endpoint (defaults to https://api.opusdns.com). Set via TF_VAR_opusdns_api_endpoint."
  type        = string
  default     = null
}

# ---------------------------------------------------------------------------
# Configure the provider with an API key.
# Provide values via TF_VAR_opusdns_api_key, a *.tfvars file, `-var`, or the
# native OPUSDNS_API_KEY / OPUSDNS_API_ENDPOINT environment variables.
# ---------------------------------------------------------------------------
provider "opusdns" {
  api_key = var.opusdns_api_key

  # Optional: override the API endpoint (e.g. for staging or local dev).
  api_endpoint = var.opusdns_api_endpoint
}
