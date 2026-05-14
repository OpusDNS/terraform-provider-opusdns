# Fetch detailed information for a single TLD.
data "opusdns_tld" "com" {
  name = "com"
}

output "com_register_price" {
  value = data.opusdns_tld.com.pricing.register_price
}

output "com_dnssec_supported" {
  value = data.opusdns_tld.com.dnssec_supported
}

output "com_contact_roles" {
  value = [for c in data.opusdns_tld.com.contact_config : c.type]
}
