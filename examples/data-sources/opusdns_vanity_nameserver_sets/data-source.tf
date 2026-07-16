data "opusdns_vanity_nameserver_sets" "all" {}

output "vanity_set_names" {
  value = [for set in data.opusdns_vanity_nameserver_sets.all.sets : set.name]
}
