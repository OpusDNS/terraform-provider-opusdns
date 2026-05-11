resource "opusdns_contact" "registrant" {
  first_name  = "Jane"
  last_name   = "Doe"
  email       = "jane.doe@example.com"
  phone       = "+1.5555550100"
  street      = "123 Main St"
  city        = "Springfield"
  state       = "IL"
  postal_code = "62701"
  country     = "US"
}

resource "opusdns_domain" "example" {
  name         = "example.com"
  period       = 1
  renewal_mode = "renew"

  contacts = {
    registrant = opusdns_contact.registrant.contact_id
    admin      = opusdns_contact.registrant.contact_id
    tech       = opusdns_contact.registrant.contact_id
    billing    = opusdns_contact.registrant.contact_id
  }

  nameservers = [
    "ns1.example.com",
    "ns2.example.com",
  ]

  transfer_lock = true
  create_zone   = true
}
