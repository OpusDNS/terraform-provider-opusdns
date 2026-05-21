data "opusdns_user_permissions" "alice" {
  user_id = opusdns_user.alice.id
}

output "alice_permissions" {
  value = data.opusdns_user_permissions.alice.permissions
}
