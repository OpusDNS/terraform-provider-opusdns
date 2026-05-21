# Link a contact to a TLD-specific attribute set.
resource "opusdns_contact_attribute_link" "hans_de" {
  contact_id               = opusdns_contact.hans.id
  contact_attribute_set_id = opusdns_contact_attribute_set.de_person.id
}
