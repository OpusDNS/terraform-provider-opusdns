# Check whether a single domain is available for registration.
data "opusdns_domain_availability" "candidate" {
  domain = "example.com"
}

output "candidate_is_available" {
  value = data.opusdns_domain_availability.candidate.is_available
}

output "candidate_status" {
  value = data.opusdns_domain_availability.candidate.status
}
