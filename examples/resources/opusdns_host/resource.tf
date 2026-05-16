# Manage a host object (glue record) for a subordinate nameserver.
# The parent domain (example.com here) must already exist in the
# authenticated organization.
resource "opusdns_host" "ns1" {
  hostname     = "ns1.example.com"
  ip_addresses = ["192.0.2.10", "2001:db8::10"]
}
