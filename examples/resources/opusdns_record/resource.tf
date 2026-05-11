resource "opusdns_record" "www" {
  zone_name = "example.com"
  name      = "www"
  type      = "A"
  ttl       = 300
  records   = ["1.2.3.4", "5.6.7.8"]
}

resource "opusdns_record" "mx" {
  zone_name = "example.com"
  name      = "@"
  type      = "MX"
  ttl       = 3600
  records   = ["10 mail.example.com.", "20 mail2.example.com."]
}

resource "opusdns_record" "txt" {
  zone_name = "example.com"
  name      = "@"
  type      = "TXT"
  ttl       = 300
  records   = ["v=spf1 include:_spf.example.com ~all"]
}
