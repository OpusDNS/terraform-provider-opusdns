# Look up by domain name.
data "opusdns_domain" "example" {
  domain_ref = "example.com"
}

# Look up by domain id.
data "opusdns_domain" "by_id" {
  domain_ref = "domain_01jxxxxxxxxxxxxxxxxxxxxxxx"
}

output "expires_on" {
  value = data.opusdns_domain.example.expires_on
}
