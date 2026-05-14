resource "opusdns_tag" "production" {
  label       = "production"
  type        = "DOMAIN"
  color       = "color-3"
  description = "Tags production-tier domains."
}
