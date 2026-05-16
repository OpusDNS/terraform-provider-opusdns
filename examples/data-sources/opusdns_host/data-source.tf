# Look up a host object by id.
data "opusdns_host" "ns1_by_id" {
  host_id = "host_01jxxxxxxxxxxxxxxxxxxxxxxx"
}

# Or look it up by hostname.
data "opusdns_host" "ns1_by_name" {
  hostname = "ns1.example.com"
}

output "ns1_ips" {
  value = data.opusdns_host.ns1_by_name.ip_addresses
}
