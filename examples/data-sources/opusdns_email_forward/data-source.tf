# Fetch a single email forward by its opaque identifier.
data "opusdns_email_forward" "info" {
  email_forward_id = "email_forward_abc123"
}

output "info_hostname" {
  value = data.opusdns_email_forward.info.hostname
}

output "info_aliases" {
  value = [for a in data.opusdns_email_forward.info.aliases : a.alias]
}
