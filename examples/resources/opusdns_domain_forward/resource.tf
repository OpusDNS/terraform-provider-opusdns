resource "opusdns_domain_forward" "www" {
  hostname = "www.example.com"
  enabled  = true

  https = [
    {
      request_path    = "/"
      target_protocol = "https"
      target_hostname = "mynewsite.com"
      target_path     = "/"
      redirect_code   = 301
    }
  ]

  http = [
    {
      request_path    = "/"
      target_protocol = "https"
      target_hostname = "mynewsite.com"
      target_path     = "/"
      redirect_code   = 301
    }
  ]
}
