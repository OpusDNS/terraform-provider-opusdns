# Manage a parking entry for a domain owned by the authenticated org.
# The organization must have accepted the parking program agreement
# (see /v1/parking/signup) before parking entries can be created.
resource "opusdns_parking" "example" {
  domain  = "example.com"
  enabled = true
}
