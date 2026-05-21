---
page_title: "Getting Started with OpusDNS"
subcategory: ""
description: |-
  Get up and running with the OpusDNS Terraform provider in minutes. Create your first DNS zone and records.
---

# Getting Started with OpusDNS

Welcome! This guide walks you through setting up the OpusDNS Terraform provider and managing your first DNS zone. By the end, you'll have a working zone with records — all managed as code. 🚀

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.0 installed
- An OpusDNS account ([sign up at opusdns.com](https://app.opusdns.com))
- An API key (we'll create one below)

## Step 1: Create an API Key

1. Log in to the [OpusDNS Dashboard](https://app.opusdns.com)
2. Navigate to **Developer** → **API Credentials**
3. Click **Create Credential**
4. Copy the generated API key — it's shown only once!

Store it safely. We recommend using environment variables:

```shell
export OPUSDNS_API_KEY="opk_your_key_here"
```

## Step 2: Configure the Provider

Create a new directory for your Terraform project and add a `main.tf`:

```terraform
terraform {
  required_providers {
    opusdns = {
      source  = "opusdns/opusdns"
      version = "~> 1.0"
    }
  }
}

provider "opusdns" {
  # Reads from OPUSDNS_API_KEY environment variable automatically.
  # You can also set it explicitly:
  # api_key = var.opusdns_api_key
}
```

## Step 3: Create Your First Zone

Add a DNS zone to `main.tf`:

```terraform
resource "opusdns_zone" "my_site" {
  name = "mysite.com"
}
```

## Step 4: Add DNS Records

Let's add the essential records for a web application:

```terraform
# Point the apex domain to your server
resource "opusdns_record" "apex" {
  zone_name = opusdns_zone.my_site.name
  name      = "@"
  type      = "A"
  ttl       = 300
  records   = ["203.0.113.50"]
}

# Point www to the same server
resource "opusdns_record" "www" {
  zone_name = opusdns_zone.my_site.name
  name      = "www"
  type      = "CNAME"
  ttl       = 300
  records   = ["mysite.com."]
}

# Add an MX record for email
resource "opusdns_record" "mx" {
  zone_name = opusdns_zone.my_site.name
  name      = "@"
  type      = "MX"
  ttl       = 3600
  records   = ["10 mail.mysite.com."]
}
```

## Step 5: Apply!

```shell
terraform init    # Download the provider
terraform plan    # Preview changes
terraform apply   # Create the resources
```

You should see output like:

```
opusdns_zone.my_site: Creating...
opusdns_zone.my_site: Creation complete after 1s
opusdns_record.apex: Creating...
opusdns_record.apex: Creation complete after 0s
opusdns_record.www: Creating...
opusdns_record.www: Creation complete after 0s
opusdns_record.mx: Creating...
opusdns_record.mx: Creation complete after 0s

Apply complete! Resources: 4 added, 0 changed, 0 destroyed.
```

That's it! Your DNS zone and records are now live and managed by Terraform. 🎉

## Step 6: Import Existing Infrastructure

Already have zones in OpusDNS? Import them:

```shell
# Import a zone
terraform import opusdns_zone.my_site mysite.com

# Import a record set (format: zone_name/record_name/type)
terraform import opusdns_record.www mysite.com/www/CNAME
```

## What's Next?

Now that you're up and running, explore more:

- **[DNS Zones & Records Guide](/docs/guides/dns-zones-and-records)** — All record types, DNSSEC, and advanced patterns
- **[Domain Registration Guide](/docs/guides/domain-registration)** — Register and manage domains end-to-end
- **[Email & Domain Forwarding Guide](/docs/guides/email-and-domain-forwarding)** — Set up email aliases and URL redirects
- **[Team & RBAC Guide](/docs/guides/team-and-rbac)** — Manage users, roles, and organizations

## Tips & Best Practices

-> **Use variables for sensitive values.** Never hardcode API keys in `.tf` files. Use `OPUSDNS_API_KEY` or Terraform variables with a `.tfvars` file (excluded from version control).

-> **Version pin the provider.** Use `version = "~> 1.0"` to get patch updates automatically while avoiding breaking changes.

-> **Use `terraform plan` before `apply`.** Always review planned changes, especially for DNS — a wrong record can take down services.
