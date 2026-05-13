# Registry-managed DNSSEC: OpusDNS generates and publishes the DS records
# for the matching OpusDNS-hosted zone. `records` must remain unset.
resource "opusdns_domain_dnssec" "managed" {
  domain_ref = opusdns_domain.example.id
  enabled    = true
}

# BYO records: replace the registry-side DNSSEC data with the records below.
resource "opusdns_domain_dnssec" "byo" {
  domain_ref = opusdns_domain.example.id

  records = [
    {
      record_type = "ds_data"
      algorithm   = 13
      digest_type = 2
      key_tag     = 12345
      digest      = "ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890"
    },
  ]
}
