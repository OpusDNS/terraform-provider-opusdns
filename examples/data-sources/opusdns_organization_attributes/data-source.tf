# Read all attributes for the caller's organization.
data "opusdns_organization_attributes" "self" {
  me = true
}

# Limit the response to a specific subset of attribute keys.
data "opusdns_organization_attributes" "filtered" {
  me   = true
  keys = ["contact_verification_notification_email"]
}

output "all_attribute_keys" {
  value = [for a in data.opusdns_organization_attributes.self.attributes : a.key]
}

# Attribute values are arbitrary JSON; decode with jsondecode().
output "notification_email" {
  value = try(
    jsondecode(data.opusdns_organization_attributes.filtered.attributes[0].value_json),
    null,
  )
}
