resource "opusdns_record" "www" {
  zone_name = "example.com"
  name      = "www"
  type      = "A"
  ttl       = 300
  rdata     = "192.0.2.1"
}

resource "opusdns_record" "mail" {
  zone_name = "example.com"
  name      = "@"
  type      = "MX"
  ttl       = 3600
  rdata     = "10 mail.example.com."
}
