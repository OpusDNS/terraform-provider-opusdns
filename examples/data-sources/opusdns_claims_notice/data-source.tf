# Retrieve a TMCH trademark claims notice for a domain.
# Use the claims_key from data.opusdns_domain_check results.
data "opusdns_claims_notice" "example" {
  claims_key = data.opusdns_domain_check.candidates.results[0].claims_key
}

output "acceptance_hash" {
  value = data.opusdns_claims_notice.example.claims_notice_acceptance_hash
}
