data "opusdns_zones" "all" {}

output "zone_names" {
  value = [for z in data.opusdns_zones.all.zones : z.name]
}
