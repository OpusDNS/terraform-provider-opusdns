---
page_title: "Email & Domain Forwarding"
subcategory: ""
description: |-
  Set up email forwarding (aliases, wildcards) and HTTP/URL redirects with the OpusDNS Terraform provider.
---

# Email & Domain Forwarding

This guide shows how to set up email aliases, catch-all forwarding, and URL redirects — all declaratively managed with Terraform.

## Email Forwarding

Email forwarding lets you receive emails at custom addresses (like `hello@yourdomain.com`) and forward them to your actual inbox — no email server required!

### Basic Setup

```terraform
resource "opusdns_email_forward" "company" {
  hostname = "mycompany.com"
  enabled  = true

  aliases = [
    {
      alias      = "hello"
      forward_to = ["founders@gmail.com"]
    },
    {
      alias      = "support"
      forward_to = ["support-team@zendesk.com", "cto@gmail.com"]
    },
  ]
}
```

This forwards:
- `hello@mycompany.com` → `founders@gmail.com`
- `support@mycompany.com` → `support-team@zendesk.com` AND `cto@gmail.com`

### Catch-All with Wildcards

Catch emails to any address that doesn't have a specific alias:

```terraform
resource "opusdns_email_forward" "with_catchall" {
  hostname = "mycompany.com"
  enabled  = true

  aliases = [
    {
      alias      = "hello"
      forward_to = ["founders@gmail.com"]
    },
    {
      alias      = "invoices"
      forward_to = ["accounting@mycompany.com"]
    },
    {
      # Wildcard: anything not matching above goes here
      alias      = "*"
      forward_to = ["admin@mycompany.com"]
    },
  ]
}
```

Now `random-person@mycompany.com` → `admin@mycompany.com` — great for catching typos and unexpected emails.

### Department Routing

Route emails to multiple team members:

```terraform
resource "opusdns_email_forward" "departments" {
  hostname = "startup.io"
  enabled  = true

  aliases = [
    {
      alias      = "sales"
      forward_to = ["alice@startup.io", "bob@startup.io"]
    },
    {
      alias      = "engineering"
      forward_to = ["eng-team@slack.com"]
    },
    {
      alias      = "press"
      forward_to = ["ceo@startup.io", "marketing@startup.io"]
    },
    {
      alias      = "security"
      forward_to = ["security-reports@startup.io"]
    },
    {
      alias      = "*"
      forward_to = ["office-manager@startup.io"]
    },
  ]
}
```

### Subdomain Email Forwarding

Forward emails on subdomains too:

```terraform
resource "opusdns_email_forward" "blog" {
  hostname = "blog.mycompany.com"
  enabled  = true

  aliases = [
    {
      alias      = "author"
      forward_to = ["content-team@mycompany.com"]
    },
  ]
}
```

## Domain / URL Forwarding

Redirect HTTP requests from one hostname to another. Perfect for vanity URLs, domain consolidation, and www-to-apex redirects.

### Basic Redirect (301 Permanent)

```terraform
resource "opusdns_domain_forward" "www_redirect" {
  hostname = "www.mycompany.com"
  enabled  = true

  https = [
    {
      request_path    = "/"
      target_protocol = "https"
      target_hostname = "mycompany.com"
      target_path     = "/"
      redirect_code   = 301
    }
  ]

  http = [
    {
      request_path    = "/"
      target_protocol = "https"
      target_hostname = "mycompany.com"
      target_path     = "/"
      redirect_code   = 301
    }
  ]
}
```

### Vanity Domain Redirect

Redirect a short domain to your main site:

```terraform
resource "opusdns_domain_forward" "vanity" {
  hostname = "myco.io"
  enabled  = true

  https = [
    {
      request_path    = "/"
      target_protocol = "https"
      target_hostname = "mycompany.com"
      target_path     = "/"
      redirect_code   = 301
    }
  ]

  http = [
    {
      request_path    = "/"
      target_protocol = "https"
      target_hostname = "mycompany.com"
      target_path     = "/"
      redirect_code   = 301
    }
  ]
}
```

### Path-Specific Redirects

Redirect specific paths to different destinations:

```terraform
resource "opusdns_domain_forward" "paths" {
  hostname = "links.mycompany.com"
  enabled  = true

  https = [
    {
      request_path    = "/docs"
      target_protocol = "https"
      target_hostname = "docs.mycompany.com"
      target_path     = "/"
      redirect_code   = 302
    },
    {
      request_path    = "/status"
      target_protocol = "https"
      target_hostname = "status.mycompany.com"
      target_path     = "/"
      redirect_code   = 302
    },
    {
      request_path    = "/"
      target_protocol = "https"
      target_hostname = "mycompany.com"
      target_path     = "/"
      redirect_code   = 301
    },
  ]
}
```

### HTTP → HTTPS Upgrade

Force all HTTP traffic to HTTPS:

```terraform
resource "opusdns_domain_forward" "force_https" {
  hostname = "mycompany.com"
  enabled  = true

  http = [
    {
      request_path    = "/"
      target_protocol = "https"
      target_hostname = "mycompany.com"
      target_path     = "/"
      redirect_code   = 301
    }
  ]
}
```

## Combining with DNS Records

Email forwarding works alongside your DNS records. Make sure you have the right MX and TXT records too:

```terraform
resource "opusdns_zone" "company" {
  name = "mycompany.com"
}

# Email forwarding
resource "opusdns_email_forward" "company" {
  hostname = opusdns_zone.company.name
  enabled  = true

  aliases = [
    {
      alias      = "hello"
      forward_to = ["team@gmail.com"]
    },
    {
      alias      = "*"
      forward_to = ["admin@gmail.com"]
    },
  ]
}

# SPF record to authorize OpusDNS email forwarding
resource "opusdns_record" "spf" {
  zone_name = opusdns_zone.company.name
  name      = "@"
  type      = "TXT"
  ttl       = 3600
  records   = ["v=spf1 include:_spf.opusdns.com ~all"]
}
```

## Querying Existing Forwards

```terraform
# List all email forwards for a zone
data "opusdns_email_forwards" "all" {
  zone_name = "mycompany.com"
}

# List all domain forwards for a zone
data "opusdns_domain_forwards" "all" {
  zone_name = "mycompany.com"
}

# Get a specific domain forward
data "opusdns_domain_forward" "www" {
  hostname = "www.mycompany.com"
}
```

## Import

```shell
# Import email forward
terraform import opusdns_email_forward.company <email_forward_id>

# Import domain forward
terraform import opusdns_domain_forward.www www.mycompany.com
```

-> **Tip:** Use `data.opusdns_email_forwards` to discover existing forwards and their IDs before importing.

## What's Next?

- **[DNS Zones & Records](/docs/guides/dns-zones-and-records)** — Fine-tune your DNS setup
- **[Domain Registration](/docs/guides/domain-registration)** — Register domains to use with forwarding
- **[Team & RBAC](/docs/guides/team-and-rbac)** — Control who manages forwarding rules
