# List all TLDs supported by the registry.
data "opusdns_tlds" "all" {}

# List only ccTLDs with DNSSEC support.
data "opusdns_tlds" "cctlds_with_dnssec" {
  type             = "ccTLD"
  dnssec_supported = true
}

# Search by substring.
data "opusdns_tlds" "dev_like" {
  search = "dev"
}

output "all_tld_count" {
  value = length(data.opusdns_tlds.all.tlds)
}

output "cctld_dnssec_names" {
  value = [for t in data.opusdns_tlds.cctlds_with_dnssec.tlds : t.name]
}
