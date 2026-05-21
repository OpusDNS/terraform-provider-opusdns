# Create a user under the authenticated caller's organization.
# `username` and `email` are immutable; changing either replaces the user.
resource "opusdns_user" "alice" {
  username   = "alice"
  email      = "alice@example.com"
  first_name = "Alice"
  last_name  = "Anderson"
  phone      = "+12125550100"
  locale     = "en-US"
}

output "alice_user_id" {
  value = opusdns_user.alice.user_id
}
