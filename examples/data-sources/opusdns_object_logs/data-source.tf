# All updates to a specific zone, newest first.
data "opusdns_object_logs" "zone_updates" {
  object_type = "ZONE"
  object_id   = "zone_01h45ytscbebyvny4gc8cr8ma2"
  action      = "update"
  sort_by     = "created_on"
  sort_order  = "desc"
  page_size   = 25
}

output "zone_update_actors" {
  value = distinct([
    for l in data.opusdns_object_logs.zone_updates.logs : l.user_id
    if l.user_id != ""
  ])
}
