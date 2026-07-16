resource "opusdns_vanity_nameserver_set" "example" {
  name               = "Example Vanity NS"
  parent_domain_name = "example.com"
  soa_rname          = "hostmaster.example.com"

  # Hostnames are ordered; the lowest position becomes the SOA MNAME.
  hostnames = [
    "ns1.example.com",
    "ns2.example.com",
  ]

  # Make this the organization's default vanity nameserver set. Zones created
  # without an explicit vanity_nameserver_set_id inherit the default.
  is_default = true
}
