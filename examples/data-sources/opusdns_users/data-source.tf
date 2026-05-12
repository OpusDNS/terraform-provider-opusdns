# List all users in the authenticated organization (GET /v1/organizations/users).
data "opusdns_users" "all" {}

output "usernames" {
  value = [for u in data.opusdns_users.all.users : u.username]
}
