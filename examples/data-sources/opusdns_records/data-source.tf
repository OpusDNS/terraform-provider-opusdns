# All RRSets in the zone.
data "opusdns_records" "all" {
  zone_name = "example.com"
}

# All MX records.
data "opusdns_records" "mx" {
  zone_name = "example.com"
  type      = "MX"
}

# Just the A and AAAA records.
data "opusdns_records" "addresses" {
  zone_name = "example.com"
  types_in  = ["A", "AAAA"]
}

output "record_count" {
  value = length(data.opusdns_records.all.records)
}
