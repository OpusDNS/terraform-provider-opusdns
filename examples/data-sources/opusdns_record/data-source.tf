data "opusdns_record" "apex" {
  zone_name = "example.com"
  name      = "@"
  type      = "A"
}

output "apex_ips" {
  value = data.opusdns_record.apex.records
}
