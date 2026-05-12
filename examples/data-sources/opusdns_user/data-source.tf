# Look up the authenticated caller's user record via GET /v1/users/me.
data "opusdns_user" "current" {
  me = true
}

output "current_user_email" {
  value = data.opusdns_user.current.email
}

# Or look up a specific user by id.
# data "opusdns_user" "specific" {
#   user_id = "user_01jnh0yy31eqmbxyrpaa70ph70"
# }
