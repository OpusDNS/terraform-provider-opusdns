# Look up the authenticated caller's organization. The provider resolves the
# id by calling GET /v1/users/me and then GET /v1/organizations/{id}.
data "opusdns_organization" "current" {
  me = true
}

output "current_org_name" {
  value = data.opusdns_organization.current.name
}

# Or look up a specific organization by id.
# data "opusdns_organization" "specific" {
#   organization_id = "organization_01jnh0v027fz2r0pcbavf9qtyy"
# }
