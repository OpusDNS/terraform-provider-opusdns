data "opusdns_zone" "example" {
  name = "example.com"
}

output "zone_dnssec_status" {
  value = data.opusdns_zone.example.dnssec_status
}
