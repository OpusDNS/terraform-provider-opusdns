data "opusdns_vanity_nameserver_set" "example" {
  set_id = "vns_example00000000000000000"
}

output "vanity_ns_hostnames" {
  value = [for ns in data.opusdns_vanity_nameserver_set.example.nameservers : ns.hostname]
}
