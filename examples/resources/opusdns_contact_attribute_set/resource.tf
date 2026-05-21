# Create a TLD-specific contact attribute set for .de domains.
resource "opusdns_contact_attribute_set" "de_person" {
  label = "DE Natural Person"
  tld   = "de"

  attributes = {
    DE_CONTACT_TYPE = "PERSON"
  }
}
