# List every tag in the caller's organization.
data "opusdns_tags" "all" {}

# Filter by tag type and free-text search.
data "opusdns_tags" "domain_prod" {
  search    = "prod"
  tag_types = ["DOMAIN"]
}

output "all_tag_labels" {
  value = [for t in data.opusdns_tags.all.tags : t.label]
}
