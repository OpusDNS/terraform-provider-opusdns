resource "opusdns_contact" "registrant" {
  first_name  = "Jane"
  last_name   = "Doe"
  email       = "jane.doe@example.com"
  phone       = "+1.2125550199"
  street      = "123 Main St"
  city        = "New York"
  state       = "NY"
  postal_code = "10001"
  country     = "US"
}

resource "opusdns_domain" "example" {
  name         = "example.com"
  period_value = 1
  period_unit  = "y"

  renewal_mode  = "renew"
  transfer_lock = true

  contacts = {
    registrant = [opusdns_contact.registrant.contact_id]
    admin      = [opusdns_contact.registrant.contact_id]
    tech       = [opusdns_contact.registrant.contact_id]
    billing    = [opusdns_contact.registrant.contact_id]
  }

  nameservers = [
    { hostname = "ns1.opusdns.com" },
    { hostname = "ns2.opusdns.com" },
  ]
}

# Transfer an existing domain in from another registrar. Set `transfer = true`
# and supply the EPP auth code from the losing registrar. `create_zone` is not
# supported for transfers, and `period_unit` must be "y" (a year count).
resource "opusdns_domain" "transferred" {
  name      = "transfer-me.com"
  transfer  = true
  auth_code = var.transfer_auth_code

  period_value = 1
  period_unit  = "y"

  contacts = {
    registrant = [opusdns_contact.registrant.contact_id]
    admin      = [opusdns_contact.registrant.contact_id]
    tech       = [opusdns_contact.registrant.contact_id]
    billing    = [opusdns_contact.registrant.contact_id]
  }
}

# Register a premium domain, confirming the expected price, and accept a TMCH
# claims notice during the claims phase. Both fields are create-only: they are
# sent with the registration request and are never read back into state.
resource "opusdns_domain" "premium" {
  name = "premium.example"

  expected_price                = 149.99
  claims_notice_acceptance_hash = var.claims_notice_acceptance_hash

  contacts = {
    registrant = [opusdns_contact.registrant.contact_id]
  }
}
