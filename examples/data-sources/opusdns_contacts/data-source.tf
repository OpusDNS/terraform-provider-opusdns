# List all contacts in the caller's organization.
data "opusdns_contacts" "all" {}

# Filter server-side: only US contacts whose last name is "Smith".
data "opusdns_contacts" "us_smiths" {
  country   = "US"
  last_name = "Smith"
}

output "contact_emails" {
  value = [for c in data.opusdns_contacts.us_smiths.contacts : c.email]
}
