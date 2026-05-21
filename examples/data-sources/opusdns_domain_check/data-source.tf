# Bulk availability check with premium and trademark info.
data "opusdns_domain_check" "candidates" {
  domains = ["example.com", "example.org", "example.io"]
}

output "results" {
  value = data.opusdns_domain_check.candidates.results
}
