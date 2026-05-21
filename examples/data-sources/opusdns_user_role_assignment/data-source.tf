data "opusdns_user_role_assignment" "alice" {
  user_id = opusdns_user.alice.id
}

output "alice_roles" {
  value = data.opusdns_user_role_assignment.alice.roles
}
