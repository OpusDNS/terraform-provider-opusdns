resource "opusdns_zone" "example" {
  name           = "example.com"
  dnssec_enabled = true
}
