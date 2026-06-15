# Delivery logs for a single email forward.
data "opusdns_email_forward_logs" "primary" {
  email_forward_id = "efw_01h45ytscbebyvny4gc8cr8ma2"
}

output "email_forward_final_statuses" {
  value = [
    for l in data.opusdns_email_forward_logs.primary.logs :
    "${l.recipient_email} -> ${l.forward_email}: ${l.final_status}"
  ]
}

# Alternative: read logs for a specific alias instead. Exactly one of
# email_forward_id / email_forward_alias_id must be set.
# data "opusdns_email_forward_logs" "alias" {
#   email_forward_alias_id = "efa_01h45ytscbebyvny4gc8cr8ma2"
# }
