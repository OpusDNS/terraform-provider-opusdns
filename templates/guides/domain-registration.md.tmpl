---
page_title: "Domain Registration"
subcategory: ""
description: |-
  Register, configure, and manage domains with the OpusDNS Terraform provider. Covers availability checks, contacts, DNSSEC, and renewal.
---

# Domain Registration

This guide covers the full domain lifecycle: checking availability, creating contacts, registering domains, enabling DNSSEC, and managing renewals — all as code.

## Check Domain Availability

Before registering, verify the domain is available:

```terraform
# Quick single-domain check
data "opusdns_domain_availability" "candidate" {
  domain = "my-awesome-startup.com"
}

output "is_available" {
  value = data.opusdns_domain_availability.candidate.is_available
}
```

For bulk checks with premium pricing info:

```terraform
data "opusdns_domain_check" "candidates" {
  domains = [
    "my-awesome-startup.com",
    "my-awesome-startup.io",
    "my-awesome-startup.dev",
  ]
}

output "availability" {
  value = {
    for r in data.opusdns_domain_check.candidates.results :
    r.domain => {
      available  = r.status == "available"
      is_premium = r.is_premium
    }
  }
}
```

## Create Contacts

Every domain registration requires contacts. Create them once and reuse across multiple domains:

```terraform
resource "opusdns_contact" "company" {
  first_name  = "Jane"
  last_name   = "Doe"
  email       = "domains@mycompany.com"
  phone       = "+12125550199"
  street      = "123 Main Street"
  city        = "New York"
  state       = "NY"
  postal_code = "10001"
  country     = "US"
  company     = "My Company Inc."
}
```

-> **Note:** Contact fields are immutable after creation. Changing any field will recreate the contact.

### Country-Specific Attributes (ccTLDs)

Some TLDs require extra attributes. For example, `.de` domains need a contact type:

```terraform
resource "opusdns_contact_attribute_set" "de_person" {
  label = "DE Natural Person"
  tld   = "de"

  attributes = {
    DE_CONTACT_TYPE = "PERSON"
  }
}

resource "opusdns_contact_attribute_link" "company_de" {
  contact_id               = opusdns_contact.company.id
  contact_attribute_set_id = opusdns_contact_attribute_set.de_person.id
}
```

## Register a Domain

With contacts ready, register your domain:

```terraform
resource "opusdns_domain" "startup" {
  name         = "my-awesome-startup.com"
  period_value = 1
  period_unit  = "y"

  contacts = {
    registrant = [opusdns_contact.company.contact_id]
    admin      = [opusdns_contact.company.contact_id]
    tech       = [opusdns_contact.company.contact_id]
    billing    = [opusdns_contact.company.contact_id]
  }

  nameservers = [
    { hostname = "ns1.opusdns.com" },
    { hostname = "ns2.opusdns.com" },
  ]

  create_zone   = true
  renewal_mode  = "renew"
  transfer_lock = true
}
```

### Key Attributes

| Attribute | Description |
|-----------|-------------|
| `period_value` + `period_unit` | Registration period (e.g., `1` + `y` = 1 year) |
| `create_zone` | Automatically create a DNS zone for this domain |
| `renewal_mode` | `renew` (auto-renew), `delete` (let expire), or `once` (renew once) |
| `transfer_lock` | Prevent unauthorized domain transfers |

## Enable DNSSEC

Secure your domain with DNSSEC after registration:

```terraform
resource "opusdns_domain_dnssec" "startup" {
  domain_ref = opusdns_domain.startup.id
  enabled    = true
}
```

This tells OpusDNS to generate and publish DS records at the registry automatically.

## Trademark / Claims Flow

Registering a domain with a trademark claim? Use the claims notice flow:

```terraform
# Step 1: Check domain — get claims_key if it exists
data "opusdns_domain_check" "trademarked" {
  domains = ["brand-name.com"]
}

# Step 2: Retrieve claims notice (only if claims_key is present)
data "opusdns_claims_notice" "notice" {
  claims_key = data.opusdns_domain_check.trademarked.results[0].claims_key
}

# Step 3: Register with acceptance hash
resource "opusdns_domain" "trademarked" {
  name         = "brand-name.com"
  period_value = 1
  period_unit  = "y"

  claims_notice_acceptance_hash = data.opusdns_claims_notice.notice.claims_notice_acceptance_hash

  contacts = {
    registrant = [opusdns_contact.company.contact_id]
    admin      = [opusdns_contact.company.contact_id]
    tech       = [opusdns_contact.company.contact_id]
    billing    = [opusdns_contact.company.contact_id]
  }

  nameservers = [
    { hostname = "ns1.opusdns.com" },
    { hostname = "ns2.opusdns.com" },
  ]

  renewal_mode  = "renew"
  transfer_lock = true
}
```

## Query TLD Information

Explore what TLDs are available and their requirements:

```terraform
# Get details for a specific TLD
data "opusdns_tld" "io" {
  name = "io"
}

# Find all TLDs that support DNSSEC
data "opusdns_tlds" "with_dnssec" {
  dnssec_supported = true
}

output "dnssec_tlds" {
  value = [for t in data.opusdns_tlds.with_dnssec.tlds : t.name]
}
```

## Managing a Domain Portfolio

Use `for_each` to manage multiple domains consistently:

```terraform
variable "domains" {
  default = {
    "mycompany.com" = { renew = true }
    "mycompany.io"  = { renew = true }
    "mycompany.dev" = { renew = true }
    "old-brand.com" = { renew = false }
  }
}

resource "opusdns_domain" "portfolio" {
  for_each = var.domains

  name         = each.key
  period_value = 1
  period_unit  = "y"

  contacts = {
    registrant = [opusdns_contact.company.contact_id]
    admin      = [opusdns_contact.company.contact_id]
    tech       = [opusdns_contact.company.contact_id]
    billing    = [opusdns_contact.company.contact_id]
  }

  nameservers = [
    { hostname = "ns1.opusdns.com" },
    { hostname = "ns2.opusdns.com" },
  ]

  renewal_mode  = each.value.renew ? "renew" : "delete"
  transfer_lock = true
}
```

## Glue Records (Hosts)

If you run your own nameservers under your domain, create host (glue) objects:

```terraform
resource "opusdns_host" "ns1" {
  hostname     = "ns1.example.com"
  ip_addresses = ["192.0.2.1", "2001:db8::1"]
}

resource "opusdns_host" "ns2" {
  hostname     = "ns2.example.com"
  ip_addresses = ["192.0.2.2", "2001:db8::2"]
}
```

Then reference them in your domain's nameservers:

```terraform
resource "opusdns_domain" "with_glue" {
  name = "example.com"
  # ...

  nameservers = [
    { hostname = opusdns_host.ns1.hostname },
    { hostname = opusdns_host.ns2.hostname },
  ]
}
```

## Multi-Registrar Setup

Connect external registrars and manage domains across providers:

```terraform
resource "opusdns_registrar_credential" "internetx" {
  name      = "InternetX Production"
  registrar = "INTERNETX"

  credentials = {
    username = "acme-corp"
    password = var.internetx_password
  }
}
```

## Import Existing Domains

```shell
terraform import opusdns_domain.startup <domain_id>
terraform import opusdns_contact.company <contact_id>
terraform import opusdns_domain_dnssec.startup example.com
```

## What's Next?

- **[DNS Zones & Records](/docs/guides/dns-zones-and-records)** — Configure DNS for your registered domains
- **[Email & Domain Forwarding](/docs/guides/email-and-domain-forwarding)** — Set up email and URL forwarding
- **[Team & RBAC](/docs/guides/team-and-rbac)** — Control who can manage your domains
