data "opusdns_reports" "completed_inventory" {
  report_types = ["domain_inventory"]
  statuses     = ["completed"]
}
