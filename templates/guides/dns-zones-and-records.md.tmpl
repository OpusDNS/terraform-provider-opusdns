---
page_title: "DNS Zones & Records"
subcategory: ""
description: |-
  Manage DNS zones, all record types, DNSSEC, and advanced patterns with the OpusDNS provider.
---

# DNS Zones & Records

This guide covers everything you need to manage DNS with Terraform: zone lifecycle, all supported record types, DNSSEC configuration, and advanced patterns like `for_each` for managing records at scale.

## Zone Lifecycle

### Creating a Zone

```terraform
resource "opusdns_zone" "production" {
  name          = "production.example.com"
  dnssec_status = "enabled"
}
```

The `dnssec_status` attribute controls whether DNSSEC signing is active for the zone. Valid values are `enabled` and `disabled`.

### Reading Zone Information

```terraform
# Look up an existing zone
data "opusdns_zone" "existing" {
  name = "example.com"
}

output "zone_status" {
  value = data.opusdns_zone.existing.dnssec_status
}

# List all zones in your account
data "opusdns_zones" "all" {}

output "all_zone_names" {
  value = [for z in data.opusdns_zones.all.zones : z.name]
}
```

### Importing an Existing Zone

```shell
terraform import opusdns_zone.production production.example.com
```

## Record Types

The `opusdns_record` resource manages a full **RRSet** — all records sharing the same name and type in a zone. Here's a comprehensive reference of supported types:

### A / AAAA — Web Hosting

```terraform
# IPv4
resource "opusdns_record" "web_v4" {
  zone_name = "example.com"
  name      = "www"
  type      = "A"
  ttl       = 300
  records   = ["203.0.113.50", "203.0.113.51"]
}

# IPv6
resource "opusdns_record" "web_v6" {
  zone_name = "example.com"
  name      = "www"
  type      = "AAAA"
  ttl       = 300
  records   = ["2001:db8::1", "2001:db8::2"]
}
```

### CNAME — Aliases

```terraform
resource "opusdns_record" "blog" {
  zone_name = "example.com"
  name      = "blog"
  type      = "CNAME"
  ttl       = 3600
  records   = ["example.github.io."]
}
```

-> **Note:** CNAME records require a trailing dot on the target FQDN.

### MX — Email Routing

```terraform
resource "opusdns_record" "mail" {
  zone_name = "example.com"
  name      = "@"
  type      = "MX"
  ttl       = 3600
  records = [
    "10 mail1.example.com.",
    "20 mail2.example.com.",
    "30 mail-backup.example.com.",
  ]
}
```

### TXT — SPF, DKIM, DMARC & Verification

```terraform
# SPF record
resource "opusdns_record" "spf" {
  zone_name = "example.com"
  name      = "@"
  type      = "TXT"
  ttl       = 3600
  records   = ["v=spf1 include:_spf.google.com include:sendgrid.net ~all"]
}

# DKIM record
resource "opusdns_record" "dkim" {
  zone_name = "example.com"
  name      = "google._domainkey"
  type      = "TXT"
  ttl       = 3600
  records   = ["v=DKIM1; k=rsa; p=MIIBIjANBgkqh..."]
}

# DMARC policy
resource "opusdns_record" "dmarc" {
  zone_name = "example.com"
  name      = "_dmarc"
  type      = "TXT"
  ttl       = 3600
  records   = ["v=DMARC1; p=reject; rua=mailto:dmarc@example.com; pct=100"]
}

# Domain verification (e.g., Google Search Console)
resource "opusdns_record" "verification" {
  zone_name = "example.com"
  name      = "@"
  type      = "TXT"
  ttl       = 300
  records   = ["google-site-verification=abc123..."]
}
```

### SRV — Service Discovery

```terraform
# Microsoft 365 Autodiscover
resource "opusdns_record" "autodiscover" {
  zone_name = "example.com"
  name      = "_autodiscover._tcp"
  type      = "SRV"
  ttl       = 3600
  records   = ["0 0 443 autodiscover.outlook.com."]
}

# SIP over TLS
resource "opusdns_record" "sip" {
  zone_name = "example.com"
  name      = "_sip._tls"
  type      = "SRV"
  ttl       = 3600
  records   = ["10 60 5061 sipdir.online.lync.com."]
}
```

### CAA — Certificate Authority Authorization

```terraform
resource "opusdns_record" "caa" {
  zone_name = "example.com"
  name      = "@"
  type      = "CAA"
  ttl       = 3600
  records = [
    "0 issue \"letsencrypt.org\"",
    "0 issuewild \"letsencrypt.org\"",
    "0 iodef \"mailto:security@example.com\"",
  ]
}
```

### NS — Delegation

```terraform
resource "opusdns_record" "subdomain_delegation" {
  zone_name = "example.com"
  name      = "internal"
  type      = "NS"
  ttl       = 86400
  records = [
    "ns1.internal-dns.example.com.",
    "ns2.internal-dns.example.com.",
  ]
}
```

### HTTPS / SVCB — Modern Service Binding

```terraform
resource "opusdns_record" "https" {
  zone_name = "example.com"
  name      = "@"
  type      = "HTTPS"
  ttl       = 300
  records   = ["1 . alpn=\"h2,h3\" ipv4hint=203.0.113.50 ipv6hint=2001:db8::1"]
}
```

### TLSA — DANE TLS Authentication

```terraform
resource "opusdns_record" "tlsa" {
  zone_name = "example.com"
  name      = "_443._tcp.www"
  type      = "TLSA"
  ttl       = 3600
  records   = ["3 1 1 2bb183af2b8..."]
}
```

### DS — Delegation Signer (for child zones)

```terraform
resource "opusdns_record" "ds" {
  zone_name = "example.com"
  name      = "secure-subdomain"
  type      = "DS"
  ttl       = 3600
  records   = ["12345 13 2 ABCDEF0123456789..."]
}
```

## DNSSEC

OpusDNS supports two DNSSEC modes at the domain level:

### Registry-Managed (Recommended)

OpusDNS generates and publishes DS records automatically:

```terraform
resource "opusdns_domain_dnssec" "auto" {
  domain_ref = opusdns_domain.example.id
  enabled    = true
}
```

### Bring Your Own DS Records

For advanced setups where you manage signing externally:

```terraform
resource "opusdns_domain_dnssec" "custom" {
  domain_ref = opusdns_domain.example.id
  enabled    = false

  records = [
    {
      record_type = "ds_data"
      algorithm   = 13
      digest_type = 2
      key_tag     = 12345
      digest      = "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"
    },
  ]
}
```

## Advanced Patterns

### Managing Multiple Records with `for_each`

Define all your records in a local map and create them dynamically:

```terraform
locals {
  dns_records = {
    "www-A" = {
      name    = "www"
      type    = "A"
      ttl     = 300
      records = ["203.0.113.50"]
    }
    "blog-CNAME" = {
      name    = "blog"
      type    = "CNAME"
      ttl     = 3600
      records = ["example.github.io."]
    }
    "mail-MX" = {
      name    = "@"
      type    = "MX"
      ttl     = 3600
      records = ["10 mail.example.com.", "20 mail2.example.com."]
    }
    "apex-TXT" = {
      name    = "@"
      type    = "TXT"
      ttl     = 3600
      records = ["v=spf1 include:_spf.google.com ~all"]
    }
  }
}

resource "opusdns_record" "managed" {
  for_each = local.dns_records

  zone_name = opusdns_zone.example.name
  name      = each.value.name
  type      = each.value.type
  ttl       = each.value.ttl
  records   = each.value.records
}
```

### Multi-Zone Setup

Manage multiple zones with shared record patterns:

```terraform
variable "zones" {
  default = ["app1.example.com", "app2.example.com", "app3.example.com"]
}

resource "opusdns_zone" "apps" {
  for_each = toset(var.zones)
  name     = each.value
}

# Every zone gets the same base records
resource "opusdns_record" "app_apex" {
  for_each = opusdns_zone.apps

  zone_name = each.value.name
  name      = "@"
  type      = "A"
  ttl       = 300
  records   = ["203.0.113.50"]
}
```

### Reading Existing Records

Query your zone's current state:

```terraform
# Get all records in a zone
data "opusdns_records" "all" {
  zone_name = "example.com"
}

# Filter by type
data "opusdns_records" "addresses" {
  zone_name = "example.com"
  types_in  = ["A", "AAAA"]
}

# Get a specific record set
data "opusdns_record" "www" {
  zone_name = "example.com"
  name      = "www"
  type      = "A"
}
```

## Import

Import existing records into Terraform state:

```shell
# Format: zone_name/record_name/record_type
terraform import opusdns_record.www example.com/www/A
terraform import opusdns_record.mail example.com/@/MX
terraform import opusdns_record.spf example.com/@/TXT
```

-> **Tip:** Use `data.opusdns_records` to discover all records in a zone before importing them.

## What's Next?

- **[Domain Registration](/docs/guides/domain-registration)** — Register domains and link them to your zones
- **[Email & Domain Forwarding](/docs/guides/email-and-domain-forwarding)** — Set up email aliases and URL redirects
- **[Getting Started](/docs/guides/getting-started)** — Back to basics
