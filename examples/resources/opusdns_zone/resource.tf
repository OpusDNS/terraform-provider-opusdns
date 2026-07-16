resource "opusdns_zone" "example" {
  name          = "example.com"
  dnssec_status = "disabled"
}

# Brand a zone's apex NS + SOA with a vanity nameserver set. When omitted, the
# organization's default vanity nameserver set (if any) is applied by the API.
resource "opusdns_zone" "branded" {
  name                     = "branded.example.com"
  vanity_nameserver_set_id = opusdns_vanity_nameserver_set.example.set_id
}
