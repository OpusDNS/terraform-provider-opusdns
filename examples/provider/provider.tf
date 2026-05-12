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
#       export TF_VAR_opusdns_client_secret=cs_xxx
#       export TF_VAR_opusdns_org_id=organization_xxx
#
# As an additional convenience, the provider itself also reads native
# `OPUSDNS_*` environment variables (OPUSDNS_ORG_ID, OPUSDNS_CLIENT_SECRET,
# OPUSDNS_USERNAME, OPUSDNS_PASSWORD, OPUSDNS_API_KEY, OPUSDNS_API_ENDPOINT)
# when the corresponding attribute is left null.
# ---------------------------------------------------------------------------

variable "opusdns_org_id" {
  description = "OpusDNS organization id (used as client_id in the client_credentials grant). Set via TF_VAR_opusdns_org_id."
  type        = string
  default     = null
}

variable "opusdns_client_secret" {
  description = "OpusDNS pre-minted client_secret. Set via TF_VAR_opusdns_client_secret."
  type        = string
  default     = null
  sensitive   = true
}

variable "opusdns_api_key" {
  description = "OpusDNS pre-minted api_key (optional companion to client_secret). Set via TF_VAR_opusdns_api_key."
  type        = string
  default     = null
  sensitive   = true
}

variable "opusdns_username" {
  description = "OpusDNS username for the password grant fallback. Set via TF_VAR_opusdns_username."
  type        = string
  default     = null
}

variable "opusdns_password" {
  description = "OpusDNS password for the password grant fallback. Set via TF_VAR_opusdns_password."
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
# Option 1 (preferred): pre-minted client credentials.
#
# Supply org_id (the client_id) and client_secret returned by
# POST /v1/auth/client_credentials. The provider performs only the final
# /v1/auth/token (grant_type=client_credentials) exchange to obtain a
# short-lived bearer access token.
#
# Provide values via TF_VAR_opusdns_org_id / TF_VAR_opusdns_client_secret,
# a *.tfvars file, or `-var` on the command line. Any attribute left null
# will fall back to the corresponding OPUSDNS_* env var inside the provider.
# ---------------------------------------------------------------------------
provider "opusdns" {
  org_id        = var.opusdns_org_id
  client_secret = var.opusdns_client_secret

  # Optional: the api_key minted alongside the client_secret. Accepted for
  # completeness/logging; the token exchange itself only needs client_secret.
  api_key = var.opusdns_api_key

  # Optional: override the API endpoint (e.g. for staging or local dev).
  api_endpoint = var.opusdns_api_endpoint
}

# ---------------------------------------------------------------------------
# Option 2 (fallback): user password grant + client_credentials bootstrap.
#
# Supply username, password, and org_id. The provider runs the full 3-step
# flow described in api/dev-resources/neovim-api-requests/api-key-connect-test.http:
#   1. POST /v1/auth/token              (grant_type=password)           -> user token
#   2. POST /v1/auth/client_credentials (Bearer user token)             -> api_key + client_secret
#   3. POST /v1/auth/token              (grant_type=client_credentials) -> bearer access token
#
# A new API key is minted on every `terraform` invocation, so prefer
# Option 1 for automation.
#
# Provide values via TF_VAR_opusdns_username / TF_VAR_opusdns_password /
# TF_VAR_opusdns_org_id, or any other Terraform variable mechanism.
# ---------------------------------------------------------------------------
# provider "opusdns" {
#   username     = var.opusdns_username
#   password     = var.opusdns_password
#   org_id       = var.opusdns_org_id
#   api_endpoint = var.opusdns_api_endpoint
# }

# ---------------------------------------------------------------------------
# Option 3: user-token (single-step password grant).
#
# Supply only username and password (omit org_id and client_secret). The
# provider performs a single POST /v1/auth/token (grant_type=password) and
# uses the resulting user access_token directly as the Authorization: Bearer
# header. The user's organization is derived from the JWT `oid` claim, so
# resources/data sources scoped to "the caller's org" work without an
# explicit org_id. Use this for endpoints documented to accept either a
# user token or client_id+client_secret. See
# api/dev-resources/neovim-api-requests/auth-login.http.
# ---------------------------------------------------------------------------
# provider "opusdns" {
#   username     = var.opusdns_username
#   password     = var.opusdns_password
#   api_endpoint = var.opusdns_api_endpoint
# }
