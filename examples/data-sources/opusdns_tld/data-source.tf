# Fetch the specification of a single TLD.
data "opusdns_tld" "com" {
  name = "com"
}

output "com_dnssec_supported" {
  value = data.opusdns_tld.com.dnssec_supported
}

output "com_registration_periods" {
  value = [for p in data.opusdns_tld.com.registration_periods : "${p.value}${p.unit}"]
}

output "com_contact_roles" {
  value = [for c in data.opusdns_tld.com.contact_config : c.type]
}
