# Look up a single contact by id.
data "opusdns_contact" "registrant" {
  contact_id = "contact_01jxxxxxxxxxxxxxxxxxxxxxxx"
}

output "registrant_email" {
  value = data.opusdns_contact.registrant.email
}
