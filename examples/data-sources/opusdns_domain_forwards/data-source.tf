# List all domain forwards configured for a zone.
data "opusdns_domain_forwards" "all" {
  zone_name = "example.com"
}

output "domain_forward_count" {
  value = length(data.opusdns_domain_forwards.all.domain_forwards)
}

output "domain_forward_hostnames" {
  value = [for f in data.opusdns_domain_forwards.all.domain_forwards : f.hostname]
}
