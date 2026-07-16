data "opusdns_role_permissions" "all" {}

# Use the catalog to build a custom role that grants read-only access to
# domains and DNS.
resource "opusdns_role" "support_read_only" {
  name        = "Support Read Only"
  description = "Read-only access for the support team."

  permissions = [
    "domains:read",
    "dns:read",
  ]
}
