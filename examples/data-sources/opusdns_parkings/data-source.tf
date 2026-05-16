# List all parking entries in the organization.
data "opusdns_parkings" "all" {}

# Filter: only enabled, approved entries, sorted by domain ascending.
data "opusdns_parkings" "approved" {
  enabled           = true
  compliance_status = "approved"
  sort_by           = "domain"
  sort_order        = "asc"
}

output "approved_domains" {
  value = [for p in data.opusdns_parkings.approved.parkings : p.domain]
}
