data "opusdns_roles" "all" {}

output "available_roles" {
  value = data.opusdns_roles.all.role_names
}
