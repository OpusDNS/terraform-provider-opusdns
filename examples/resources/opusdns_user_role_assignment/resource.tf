resource "opusdns_user_role_assignment" "alice" {
  user_id = opusdns_user.alice.id

  roles = [
    "member",
    "domain_manager",
    "contact_manager",
  ]
}
