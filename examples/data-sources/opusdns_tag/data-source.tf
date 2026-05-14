# Look up a single tag by id.
data "opusdns_tag" "production" {
  tag_id = "tag_01jxxxxxxxxxxxxxxxxxxxxxxx"
}

output "production_label" {
  value = data.opusdns_tag.production.label
}
