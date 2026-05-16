# Look up a parking entry by id.
data "opusdns_parking" "by_id" {
  parking_id = "parking_01jxxxxxxxxxxxxxxxxxxxxxxx"
}

# Or look it up by domain name.
data "opusdns_parking" "by_domain" {
  domain = "example.com"
}

output "example_enabled" {
  value = data.opusdns_parking.by_domain.enabled
}
