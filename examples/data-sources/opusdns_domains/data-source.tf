# List all domains in the caller's organization.
data "opusdns_domains" "all" {}

# Filter server-side: .com domains with the transfer lock on, set to auto-renew.
data "opusdns_domains" "locked_com" {
  tld           = "com"
  transfer_lock = true
  renewal_mode  = "renew"
}

output "domain_names" {
  value = [for d in data.opusdns_domains.all.domains : d.name]
}
