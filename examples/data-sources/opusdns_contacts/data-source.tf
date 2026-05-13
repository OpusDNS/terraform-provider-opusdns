# List all contacts in the caller's organization.
data "opusdns_contacts" "all" {}

# Filter server-side: only verified US contacts whose last name is "Smith".
data "opusdns_contacts" "verified_us_smiths" {
  country   = "US"
  last_name = "Smith"
  verified  = true
}

output "contact_emails" {
  value = [for c in data.opusdns_contacts.contact_list.contacts : c.email]
}
