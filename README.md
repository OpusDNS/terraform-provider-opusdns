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

# Values can also be supplied via OPUSDNS_API_KEY (or any TF_VAR_* mechanism
# if wired through Terraform variables).
provider "opusdns" {
  api_key = var.opusdns_api_key
}
```

See [`examples/provider/provider.tf`](examples/provider/provider.tf) for a full example wired through Terraform variables.

### Provider Configuration

The provider authenticates with an OpusDNS API key sent via the `X-Api-Key` header on every request.

**Breaking change:** `org_id`, `client_secret`, `username`, and `password` are no longer supported provider arguments. Configure the provider with `api_key` only.

| Attribute       | Type   | Required             | Env var                 | Description |
|-----------------|--------|----------------------|-------------------------|-------------|
| `api_key`       | string | Yes                  | `OPUSDNS_API_KEY`       | OpusDNS API key. |
| `api_endpoint`  | string | No                   | `OPUSDNS_API_ENDPOINT`  | Override the API endpoint (defaults to `https://api.opusdns.com`). |

#### Creating an API key

Create an API key from the OpusDNS dashboard:

1. Log in to your OpusDNS account at <https://app.opusdns.com>.
2. Navigate to **Developer** > **API Credentials**.
3. Create a new credential and copy the generated API key.
4. Supply the value to the provider via `api_key` or `OPUSDNS_API_KEY`.

The API key is shown only once at creation. Store it in a secret manager and treat it like a password; it grants API access for the organization.

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

---

### `opusdns_user`

Manages a user under the authenticated caller's organization (`/v1/users`). `username` and `email` are immutable — changing either recreates the user.

```hcl
resource "opusdns_user" "alice" {
  username   = "alice"
  email      = "alice@example.com"
  first_name = "Alice"
  last_name  = "Anderson"
  phone      = "+1.2125550100"
  locale     = "en-US"
}
```

**Import:** `terraform import opusdns_user.alice <user_id>`

---

### `opusdns_tag`

Manages a tag (`/v1/tags`). Tags categorize resources (`DOMAIN`, `CONTACT`, or `ZONE`). The `type` field is immutable — changing it forces replacement; `label`, `color`, and `description` are updatable in place.

```hcl
resource "opusdns_tag" "production" {
  label       = "production"
  type        = "DOMAIN"
  color       = "color-3"
  description = "Tags production-tier domains."
}
```

**Import:** `terraform import opusdns_tag.production <tag_id>`

---

### `opusdns_domain`

Registers and manages a domain (`/v1/domains`). `contacts`, `nameservers`, `renewal_mode`, and `transfer_lock` are updatable in place; all other inputs (name, period, `create_zone`, `auth_code`) force replacement.

```hcl
resource "opusdns_domain" "example" {
  name = "example.com"

  contacts = {
    registrant = [opusdns_contact.admin.id]
    admin      = [opusdns_contact.admin.id]
    tech       = [opusdns_contact.admin.id]
    billing    = [opusdns_contact.admin.id]
  }

  period_value  = 1
  period_unit   = "y"
  create_zone   = true
  renewal_mode  = "renew"
  transfer_lock = true

  nameservers = [
    { hostname = "ns1.opusdns.com" },
    { hostname = "ns2.opusdns.com" },
  ]
}
```

**Import:** `terraform import opusdns_domain.example <domain_id>`

---

### `opusdns_domain_dnssec`

Manages DNSSEC configuration for a domain. Two mutually exclusive modes: registry-managed (`enabled = true`, OpusDNS generates and publishes DS records) or bring-your-own (`enabled = false` with explicit `records`).

```hcl
# Registry-managed
resource "opusdns_domain_dnssec" "example" {
  domain_ref = "example.com"
  enabled    = true
}

# Bring-your-own DS records
resource "opusdns_domain_dnssec" "byo" {
  domain_ref = "example.com"
  enabled    = false

  records = [
    {
      record_type = "ds_data"
      algorithm   = 13
      digest_type = 2
      key_tag     = 12345
      digest      = "ABCDEF0123456789..."
    }
  ]
}
```

**Import:** `terraform import opusdns_domain_dnssec.example example.com` (accepts domain id or name; `enabled` defaults to `false` on import — set it in config to match registry-managed mode).

---

### `opusdns_host`

Manages a host (glue) object (`/v1/hosts`) binding an FQDN to one or more IP addresses. `hostname` is immutable.

```hcl
resource "opusdns_host" "ns1" {
  hostname     = "ns1.example.com"
  ip_addresses = ["192.0.2.1", "2001:db8::1"]
}
```

**Import:** `terraform import opusdns_host.ns1 <host_id>` (also accepts the hostname, e.g. `ns1.example.com`).

---

### `opusdns_parking`

Manages a parking entry (`/v1/parking`) attaching an ad-serving placeholder page to a domain. The org must have accepted the parking program agreement (`POST /v1/parking/signup`) before resources can be created. `domain` is immutable.

```hcl
resource "opusdns_parking" "example" {
  domain  = "example.com"
  enabled = true
}
```

**Import:** `terraform import opusdns_parking.example <parking_id>` (also accepts the domain name, e.g. `example.com`).

---

### `opusdns_registrar_credential`

Manages a third-party registrar credential (`/v1/connect/registrars`) used by OpusDNS Connect to act on the org's behalf at the registrar. `name`, `registrar`, and `credentials` are all immutable. The `credentials` payload is write-only — the API never returns it, so drift cannot be detected.

```hcl
resource "opusdns_registrar_credential" "internetx" {
  name      = "InternetX Production"
  registrar = "INTERNETX"

  credentials = {
    username = "acme"
    password = var.internetx_password
    # additional registrar-specific keys as required
  }
}
```

**Import:** `terraform import opusdns_registrar_credential.internetx <registrar_credential_id>` (credentials must be re-supplied in config after import).

---

### `opusdns_user_role_assignment`

Manages the full set of roles assigned to a user (`PATCH /v1/users/{user_id}/roles`). The configured `roles` set is treated as the desired total; the provider diffs against current state and issues minimal add/remove batches. `user_id` is immutable.

```hcl
resource "opusdns_user_role_assignment" "alice" {
  user_id = opusdns_user.alice.id

  roles = [
    "member",
    "domain_manager",
    "contact_manager",
  ]
}
```

Allowed roles: `admin`, `api_admin`, `billing_manager`, `chat_manager`, `cms_content_editor`, `contact_manager`, `domain_forward_manager`, `domain_manager`, `email_forward_manager`, `events_manager`, `host_manager`, `member`, `organization_manager`, `product_manager`, `registrar_credential_manager`, `reseller_manager`.

**Import:** `terraform import opusdns_user_role_assignment.alice <user_id>`

---

### `opusdns_contact_attribute_set`

Manages a TLD-scoped set of registry-specific contact attributes (`POST/GET/PATCH/DELETE /v1/contacts/attribute-sets`). Many ccTLDs (`.de`, `.no`, `.us`, `.eu`, ...) require extra fields on a contact that are not part of the base `opusdns_contact` schema; this resource captures those as a reusable, named bundle that can be linked to one or more contacts.

`tld` and `attributes` are immutable after creation (changing either recreates the set). `label` is updatable in place.

```hcl
resource "opusdns_contact_attribute_set" "de_person" {
  label = "DE persons"
  tld   = "de"

  attributes = {
    DE_CONTACT_TYPE = "PERSON"
  }
}
```

Attribute keys must come from the registry attribute enum (e.g. `DE_CONTACT_TYPE`, `NOMINET_CO_NO`, `SIDN_LEGAL_FORM`, `US_NEXUS_CATEGORY`). See the OpusDNS API reference for the full enum and per-TLD requirements.

**Note:** A set cannot be deleted while linked to any contact; destroy the linking `opusdns_contact_attribute_link` (or the contacts themselves) first.

**Import:** `terraform import opusdns_contact_attribute_set.de_person <contact_attribute_set_id>`

---

### `opusdns_contact_attribute_link`

Links a contact to a `opusdns_contact_attribute_set` (`PATCH /v1/contacts/{contact_id}/attribute-sets`).

```hcl
resource "opusdns_contact_attribute_link" "hans" {
  contact_id               = opusdns_contact.hans.id
  contact_attribute_set_id = opusdns_contact_attribute_set.de_person.id
}
```

**Caveat:** The API does not expose an unlink endpoint. On `terraform destroy` the resource is removed from state with a warning, but the link itself persists server-side until either the contact or the attribute set is deleted.

**Import:** `terraform import opusdns_contact_attribute_link.hans <contact_id>:<contact_attribute_set_id>`

---

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

### `data.opusdns_organization`

Fetches a single organization. Set `me = true` to resolve the authenticated caller's own organization (via `/v1/users/me`), or supply `organization_id` to look up `/v1/organizations/{id}`.

```hcl
data "opusdns_organization" "current" {
  me = true
}
```

### `data.opusdns_organizations`

Lists child organizations beneath the authenticated organization (`GET /v1/organizations`).

```hcl
data "opusdns_organizations" "all" {}

output "organization_names" {
  value = [for o in data.opusdns_organizations.all.organizations : o.name]
}
```

### `data.opusdns_user`

Fetches a single user. Set `me = true` for `/v1/users/me`, otherwise supply `user_id`.

```hcl
data "opusdns_user" "current" {
  me = true
}
```

### `data.opusdns_users`

Lists users in the authenticated caller's organization (`GET /v1/organizations/users`).

```hcl
data "opusdns_users" "all" {}

output "usernames" {
  value = [for u in data.opusdns_users.all.users : u.username]
}
```

### `data.opusdns_tag`

Fetches a single tag by id (`GET /v1/tags/{tag_id}`).

```hcl
data "opusdns_tag" "production" {
  tag_id = "tag_01jxxxxxxxxxxxxxxxxxxxxxxx"
}
```

### `data.opusdns_tags`

Lists tags in the authenticated caller's organization (`GET /v1/tags`). Optional `search` and `tag_types` filters narrow the server-side query.

```hcl
data "opusdns_tags" "domain_prod" {
  search    = "prod"
  tag_types = ["DOMAIN"]
}

output "tag_labels" {
  value = [for t in data.opusdns_tags.domain_prod.tags : t.label]
}
```

### `data.opusdns_user_role_assignment`

Fetches the set of provider-managed roles currently assigned to a user (`GET /v1/users/{user_id}/roles`). Implicit roles such as `accepted_tos`, `owner`, and `self` are filtered out.

```hcl
data "opusdns_user_role_assignment" "alice" {
  user_id = opusdns_user.alice.id
}
```

### `data.opusdns_contact`

Fetches a single contact by id (`GET /v1/contacts/{contact_id}`).

```hcl
data "opusdns_contact" "admin" {
  contact_id = "contact_01jxxxxxxxxxxxxxxxxxxxxxxx"
}
```

### `data.opusdns_contacts`

Lists contacts in the authenticated caller's organization (`GET /v1/contacts`). Optional `search`, `first_name`, `last_name`, `email`, `country`, and `verified` filters narrow the server-side query.

```hcl
data "opusdns_contacts" "verified_us" {
  country  = "US"
  verified = true
}

output "contact_emails" {
  value = [for c in data.opusdns_contacts.verified_us.contacts : c.email]
}
```

### `data.opusdns_domain`

Fetches a single domain (`GET /v1/domains/{ref}`). `domain_ref` may be either a `domain_...` id or an FQDN.

```hcl
data "opusdns_domain" "example" {
  domain_ref = "example.com"
}
```

### `data.opusdns_domains`

Lists domains in the authenticated caller's organization (`GET /v1/domains`). Optional `search`, `name`, `tld`, `sld`, `status`, `renewal_mode`, `transfer_lock`, and `is_premium` filters narrow the server-side query.

```hcl
data "opusdns_domains" "com" {
  tld = "com"
}

output "domain_names" {
  value = [for d in data.opusdns_domains.com.domains : d.name]
}
```

### `data.opusdns_domain_dnssec`

Reads the DNSSEC configuration for a domain. `domain_ref` may be either a `domain_...` id or an FQDN.

```hcl
data "opusdns_domain_dnssec" "example" {
  domain_ref = "example.com"
}
```

### `data.opusdns_domain_availability`

Checks whether a domain is available for registration (`GET /v1/availability`). Useful as a precondition for `opusdns_domain`.

```hcl
data "opusdns_domain_availability" "example" {
  domain = "example.com"
}

output "available" {
  value = data.opusdns_domain_availability.example.is_available
}
```

### `data.opusdns_record`

Reads a single DNS record set (RRSet) by zone, name, and type.

```hcl
data "opusdns_record" "www" {
  zone_name = "example.com"
  name      = "www"
  type      = "A"
}
```

### `data.opusdns_records`

Lists DNS record sets (RRSets) in a zone. Optional `name`, `type`, or `types_in` filters narrow the result (filtering is performed client-side; `type` and `types_in` are mutually exclusive).

```hcl
data "opusdns_records" "mx_records" {
  zone_name = "example.com"
  type      = "MX"
}

output "mx" {
  value = [for r in data.opusdns_records.mx_records.records : r.records]
}
```

### `data.opusdns_email_forward`

Fetches a single email forward by id (`GET /v1/email-forwards/{id}`).

```hcl
data "opusdns_email_forward" "example" {
  email_forward_id = "email_forward_01jxxxxxxxxxxxxxxxxxxxxxxx"
}
```

### `data.opusdns_email_forwards`

Lists email forwards configured for a zone (`GET /v1/dns/{zone}/email-forwards`).

```hcl
data "opusdns_email_forwards" "example" {
  zone_name = "example.com"
}

output "forward_hostnames" {
  value = [for f in data.opusdns_email_forwards.example.email_forwards : f.hostname]
}
```

### `data.opusdns_domain_forward`

Fetches a single domain (HTTP) forward by hostname (`GET /v1/domain-forwards/{hostname}`).

```hcl
data "opusdns_domain_forward" "www" {
  hostname = "www.example.com"
}
```

### `data.opusdns_domain_forwards`

Lists domain forwards configured for a zone (`GET /v1/dns/{zone}/domain-forwards`).

```hcl
data "opusdns_domain_forwards" "example" {
  zone_name = "example.com"
}

output "forward_hostnames" {
  value = [for f in data.opusdns_domain_forwards.example.domain_forwards : f.hostname]
}
```

### `data.opusdns_host`

Reads a single host (glue) object (`GET /v1/hosts/{ref}`). Exactly one of `host_id` or `hostname` must be supplied.

```hcl
data "opusdns_host" "ns1" {
  hostname = "ns1.example.com"
}
```

### `data.opusdns_parking`

Reads a single parking entry (`GET /v1/parking/{ref}`). Exactly one of `parking_id` or `domain` must be supplied.

```hcl
data "opusdns_parking" "example" {
  domain = "example.com"
}
```

### `data.opusdns_parkings`

Lists parking entries in the authenticated caller's organization (`GET /v1/parking`). Optional `search`, `enabled`, `compliance_status`, `sort_by`, and `sort_order` filters narrow the result.

```hcl
data "opusdns_parkings" "approved" {
  compliance_status = "approved"
}

output "parked_domains" {
  value = [for p in data.opusdns_parkings.approved.parkings : p.domain]
}
```

### `data.opusdns_registrar_credential`

Reads metadata for a single registrar credential (`GET /v1/connect/registrars/{registrar_credential_id}`). The credential payload itself is never returned by the API.

```hcl
data "opusdns_registrar_credential" "internetx" {
  registrar_credential_id = "registrar_credential_01jxxxxxxxxxxxxxxxxxxxxxxx"
}
```

### `data.opusdns_registrar_credentials`

Lists registrar credentials in the authenticated caller's organization (`GET /v1/connect/registrars`). Optional `registrar` filter narrows by provider (`INTERNETX`, `MONIKER`, `DOMAIN_BESTELLSYSTEM`, `CENTRALNIC`, `OPUSDNS`, `ENOM`).

```hcl
data "opusdns_registrar_credentials" "internetx" {
  registrar = "INTERNETX"
}

output "credential_names" {
  value = [for c in data.opusdns_registrar_credentials.internetx.registrar_credentials : c.name]
}
```

### `data.opusdns_tld`

Fetches detailed information for a single TLD (`GET /v1/tlds/{tld}`), including pricing, restrictions, contact/nameserver requirements, and launch phases.

```hcl
data "opusdns_tld" "com" {
  name = "com"
}
```

### `data.opusdns_tlds`

Lists all TLDs supported by the registry (flat fields only; use `data.opusdns_tld` for rich details). Optional `search`, `type`, `available`, `registration_enabled`, and `dnssec_supported` filters narrow the result.

```hcl
data "opusdns_tlds" "dnssec_enabled" {
  dnssec_supported = true
}

output "tld_names" {
  value = [for t in data.opusdns_tlds.dnssec_enabled.tlds : t.name]
}
```

### `data.opusdns_contact_attribute_set`

Reads a single contact attribute set by id (`GET /v1/contacts/attribute-sets/{contact_attribute_set_id}`).

```hcl
data "opusdns_contact_attribute_set" "de_person" {
  contact_attribute_set_id = "contact_attribute_set_01jxxxxxxxxxxxxxxxxxxxxxxx"
}
```

### `data.opusdns_contact_attribute_sets`

Lists contact attribute sets in the organization (`GET /v1/contacts/attribute-sets`). Optional `tld` and `label` filters narrow the result.

```hcl
data "opusdns_contact_attribute_sets" "all_de" {
  tld = "de"
}

output "set_labels" {
  value = [for s in data.opusdns_contact_attribute_sets.all_de.contact_attribute_sets : s.label]
}
```

### `data.opusdns_roles`

Lists all roles available in the caller's organization (`GET /v1/organizations/roles`). The endpoint's response shape is untyped in the OpenAPI spec, so this data source surfaces both a best-effort `role_names` list and the raw `roles_json` body.

```hcl
data "opusdns_roles" "all" {}

output "available_roles" {
  value = data.opusdns_roles.all.role_names
}
```

### `data.opusdns_user_permissions`

Reads a user's effective permission set (`GET /v1/users/{user_id}/permissions`). Returns both a `permissions` list of strings and the raw `permissions_json` for callers needing the unprocessed response.

```hcl
data "opusdns_user_permissions" "alice" {
  user_id = opusdns_user.alice.id
}

output "alice_permissions" {
  value = data.opusdns_user_permissions.alice.permissions
}
```

### `data.opusdns_domain_check`

Performs a real-time bulk availability check (`GET /v1/domains/check`). Richer than `data.opusdns_domain_availability` — also returns premium-pricing info and TMCH `claims_key`s, making it the preferred precondition for `opusdns_domain` registrations of premium or trademarked names.

```hcl
data "opusdns_domain_check" "candidates" {
  domains = ["example.com", "example.org"]
}

output "results" {
  value = data.opusdns_domain_check.candidates.results
}
```

### `data.opusdns_claims_notice`

Retrieves a TMCH trademark claims notice for a single claims key (`POST /v1/domains/claims-notices`). Use the value from `data.opusdns_domain_check.results[*].claims_key`. The returned `claims_notice_acceptance_hash` is required when registering the matching trademarked domain via `opusdns_domain`.

```hcl
data "opusdns_domain_check" "tm" {
  domains = ["acme.example"]
}

data "opusdns_claims_notice" "tm" {
  claims_key = data.opusdns_domain_check.tm.results[0].claims_key
}

output "acceptance_hash" {
  value = data.opusdns_claims_notice.tm.claims_notice_acceptance_hash
}
```

## Building from Source

```sh
git clone https://github.com/opusdns/terraform-provider-opusdns.git
cd terraform-provider-opusdns
make build
```

### Local Development

To use a locally built provider binary, add a [developer override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) to your `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    # Point at the directory containing the built `terraform-provider-opusdns`
    # binary. `make install` installs to $GOBIN (or $GOPATH/bin); find it with
    #   go env GOBIN || echo "$(go env GOPATH)/bin"
    "opusdns/opusdns" = "/Users/you/gocode/bin"
  }
  direct {}
}
```

Then run `make install` to build and install the provider binary. With `dev_overrides` active you do **not** need to run `terraform init` — Terraform will load the binary directly on every `plan`/`apply`.

## License

[Mozilla Public License 2.0](LICENSE)

## Support

- Documentation: <https://developers.opusdns.com>
- Issues: [GitHub Issues](https://github.com/OpusDNS/terraform-provider-opusdns/issues)
- Email: <support@opusdns.com>
