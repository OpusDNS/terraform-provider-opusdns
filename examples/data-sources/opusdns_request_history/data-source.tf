# Latest 50 API requests with status >= 400.
data "opusdns_request_history" "errors" {
  page_size       = 50
  sort_by         = "created_on"
  sort_order      = "desc"
  min_status_code = 400
}

output "request_history_errors" {
  value = [
    for e in data.opusdns_request_history.errors.entries :
    "${e.method} ${e.path} -> ${e.status_code} (${e.duration_ms}ms)"
  ]
}
