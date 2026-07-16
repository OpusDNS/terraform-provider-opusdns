data "opusdns_role_permissions" "all" {}

output "grantable_permissions" {
  value = data.opusdns_role_permissions.all.permissions
}
