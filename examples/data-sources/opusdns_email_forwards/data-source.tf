# List all email forwards configured for a zone.
data "opusdns_email_forwards" "all" {
  zone_name = "example.com"
}

output "email_forward_count" {
  value = length(data.opusdns_email_forwards.all.email_forwards)
}

output "email_forward_hostnames" {
  value = [for f in data.opusdns_email_forwards.all.email_forwards : f.hostname]
}
