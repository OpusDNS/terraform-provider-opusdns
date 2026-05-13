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
