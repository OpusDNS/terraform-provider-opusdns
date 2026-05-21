# List all contact attribute sets for .de TLD.
data "opusdns_contact_attribute_sets" "de" {
  tld = "de"
}

output "set_labels" {
  value = [for s in data.opusdns_contact_attribute_sets.de.contact_attribute_sets : s.label]
}
