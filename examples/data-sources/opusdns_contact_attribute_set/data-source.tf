data "opusdns_contact_attribute_set" "de_person" {
  contact_attribute_set_id = "contact_attribute_set_01jxxxxxxxxxxxxxxxxxxxxxxx"
}

output "attributes" {
  value = data.opusdns_contact_attribute_set.de_person.attributes
}
