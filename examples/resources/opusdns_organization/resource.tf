# Create a child organization beneath the authenticated caller's org.
# All address/tax fields are optional and can be updated in place; changing
# `name` etc. issues a PATCH /v1/organizations/{id}.
resource "opusdns_organization" "subsidiary" {
  name           = "Acme Subsidiary, Inc."
  address_1      = "123 Main St"
  city           = "New York"
  state          = "NY"
  postal_code    = "10001"
  country_code   = "US"
  currency       = "USD"
  default_locale = "en-US"
}

output "subsidiary_id" {
  value = opusdns_organization.subsidiary.organization_id
}
