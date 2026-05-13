data "opusdns_domain_dnssec" "example" {
  domain_ref = "example.com"
}

output "ds_records" {
  value = data.opusdns_domain_dnssec.example.records
}
