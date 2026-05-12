# List child organizations beneath the authenticated organization.
data "opusdns_organizations" "all" {}

output "organization_names" {
  value = [for o in data.opusdns_organizations.all.organizations : o.name]
}
