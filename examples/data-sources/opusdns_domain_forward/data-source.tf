# Fetch a single domain forward by its hostname.
data "opusdns_domain_forward" "www" {
  hostname = "www.example.com"
}

output "www_enabled" {
  value = data.opusdns_domain_forward.www.enabled
}

output "www_http_redirects" {
  value = [
    for r in data.opusdns_domain_forward.www.http :
    "${r.request_path} -> ${r.target_protocol}://${r.target_hostname}${r.target_path} (${r.redirect_code})"
  ]
}
