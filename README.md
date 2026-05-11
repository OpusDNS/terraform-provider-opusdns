# Terraform Provider for OpusDNS

The OpusDNS Terraform provider allows you to manage DNS zones, DNS records, contacts, email forwards, and domain forwards via the [OpusDNS API](https://docs.opusdns.com).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.26 (for building from source)

## Using the Provider

```hcl
terraform {
  required_providers {
    opusdns = {
      source  = "opusdns/opusdns"
      version = "~> 1.0"
    }
  }
}

provider "opusdns" {
  # Set via OPUSDNS_API_KEY environment variable (recommended) or inline.
  api_key = "opk_your_api_key_here"
}
```

### Provider Configuration

| Attribute      | Type   | Required | Description |
|----------------|--------|----------|-------------|
| `api_key`      | string | Yes      | Your OpusDNS API key. Can also be set via `OPUSDNS_API_KEY`. |
| `api_endpoint` | string | No       | Override the API endpoint. Can also be set via `OPUSDNS_API_ENDPOINT`. |

## Resources

### `opusdns_zone`

Manages a DNS zone.

```hcl
resource "opusdns_zone" "example" {
  name          = "example.com"
  dnssec_status = "disabled"
}
```

**Import:** `terraform import opusdns_zone.example example.com`

---

### `opusdns_record`

Manages a DNS record set (RRSet) — all records sharing the same name and type within a zone.

```hcl
resource "opusdns_record" "www" {
  zone_name = "example.com"
  name      = "www"
  type      = "A"
  ttl       = 300
  records   = ["1.2.3.4", "5.6.7.8"]
}
```

**Import:** `terraform import opusdns_record.www example.com/www/A`

Supported record types: `A`, `AAAA`, `ALIAS`, `CAA`, `CNAME`, `MX`, `NS`, `PTR`, `TXT`, `SRV`, `SSHFP`, `TLSA`, `DS`, `HTTPS`, `SVCB`, and more.

---

### `opusdns_contact`

Manages a contact used for domain registrations. All fields are immutable after creation (changing any field recreates the contact).

```hcl
resource "opusdns_contact" "admin" {
  first_name  = "Jane"
  last_name   = "Doe"
  email       = "jane.doe@example.com"
  phone       = "+1.2125550199"
  street      = "123 Main St"
  city        = "New York"
  state       = "NY"
  postal_code = "10001"
  country     = "US"
  disclose    = false
}
```

**Import:** `terraform import opusdns_contact.admin <contact_id>`

---

### `opusdns_email_forward`

Manages email forwarding for a hostname, including aliases.

```hcl
resource "opusdns_email_forward" "example" {
  hostname = "example.com"
  enabled  = true

  aliases = [
    {
      alias      = "info"
      forward_to = ["admin@mycompany.com"]
    },
    {
      alias      = "*"
      forward_to = ["catchall@mycompany.com"]
    },
  ]
}
```

**Import:** `terraform import opusdns_email_forward.example <email_forward_id>`

---

### `opusdns_domain_forward`

Manages URL/domain forwarding (HTTP redirects) for a hostname.

```hcl
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
}
```

**Import:** `terraform import opusdns_domain_forward.www www.example.com`

## Data Sources

### `data.opusdns_zone`

Fetches information about an existing DNS zone.

```hcl
data "opusdns_zone" "example" {
  name = "example.com"
}
```

### `data.opusdns_zones`

Fetches all DNS zones in your account.

```hcl
data "opusdns_zones" "all" {}

output "zone_names" {
  value = [for z in data.opusdns_zones.all.zones : z.name]
}
```

## Building from Source

```sh
git clone https://github.com/opusdns/terraform-provider-opusdns.git
cd terraform-provider-opusdns
make build
```

### Local Development

To use a locally built provider, add a [developer override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) to your `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "opusdns/opusdns" = "/path/to/terraform-provider-opusdns"
  }
  direct {}
}
```

Then run `make install` to build and install the provider binary.

## License

[Mozilla Public License 2.0](LICENSE)
