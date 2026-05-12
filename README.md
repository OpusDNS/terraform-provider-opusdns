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

# Preferred: pre-minted client credentials. Values can also be supplied via
# OPUSDNS_ORG_ID / OPUSDNS_CLIENT_SECRET (or any TF_VAR_* mechanism if wired
# through Terraform variables).
provider "opusdns" {
  org_id        = var.opusdns_org_id
  client_secret = var.opusdns_client_secret
}
```

See [`examples/provider/provider.tf`](examples/provider/provider.tf) for both auth modes wired through Terraform variables.

### Provider Configuration

The provider authenticates via the OpusDNS `/v1/auth` OAuth2 endpoints. Three modes are supported, selected in this priority order:

1. **Pre-minted client credentials (preferred for automation):** supply `org_id` + `client_secret`. The provider runs only the final `/v1/auth/token` (`grant_type=client_credentials`) exchange.
2. **Full 3-step bootstrap:** supply `username` + `password` + `org_id`. The provider runs the full flow (password grant → mint API key → client_credentials grant). A new API key is minted on every `terraform` invocation.
3. **User-token (single-step):** supply `username` + `password` only (omit `org_id` and `client_secret`). The provider performs the single `/v1/auth/token` (`grant_type=password`) call and uses the returned user access_token directly as the `Authorization: Bearer` token. The org is taken from the JWT `oid` claim. Use this for endpoints that accept either a user token or `client_id`+`client_secret`.

| Attribute       | Type   | Required             | Env var                 | Description |
|-----------------|--------|----------------------|-------------------------|-------------|
| `org_id`        | string | Modes 1, 2           | `OPUSDNS_ORG_ID`        | Organization id (used as `client_id`), e.g. `organization_...`. Omit for mode 3. |
| `client_secret` | string | Mode 1               | `OPUSDNS_CLIENT_SECRET` | Pre-minted client_secret from `/v1/auth/client_credentials`. |
| `api_key`       | string | No                   | `OPUSDNS_API_KEY`       | Pre-minted api_key (companion to client_secret; not required for the grant). |
| `username`      | string | Modes 2, 3           | `OPUSDNS_USERNAME`      | OpusDNS username for the password grant. |
| `password`      | string | Modes 2, 3           | `OPUSDNS_PASSWORD`      | OpusDNS password for the password grant. |
| `api_endpoint`  | string | No                   | `OPUSDNS_API_ENDPOINT`  | Override the API endpoint (defaults to `https://api.opusdns.com`). |

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

### `opusdns_organization`

Manages an organization under the authenticated caller's organization (the API treats the caller as the parent automatically). `name` and the address/tax fields are updatable in-place; `create` and `delete` are full lifecycle operations against `/v1/organizations`.

```hcl
resource "opusdns_organization" "subsidiary" {
  name           = "Acme Subsidiary, Inc."
  address_1      = "123 Main St"
  city           = "New York"
  state          = "NY"
  postal_code    = "10001"
  country_code   = "US"
  currency       = "USD"
  default_locale = "en-US"
}
```

**Import:** `terraform import opusdns_organization.subsidiary <organization_id>`

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

## Testing Basic Operations During Development

The fastest way to exercise the provider end-to-end while you iterate is:

### 1. Bring up a local OpusDNS API

```sh
cd ../api
# Follow the dev-resources README to start the API stack (typically
# `tilt up` or `docker compose up`). The API will be reachable at
# http://api.opusdns.local once ready.
```

The auth flow you're targeting is documented in `../api/dev-resources/neovim-api-requests/api-key-connect-test.http`.

### 2. Build & install the provider

```sh
make install         # builds and installs to $(go env GOPATH)/bin
```

Confirm `~/.terraformrc` contains the `dev_overrides` block above pointing at `$(go env GOPATH)/bin`.

### 3. Export credentials

The provider reads `OPUSDNS_*` env vars when the matching attribute is null. For local development against `api.opusdns.local`:

```sh
# Option A: pre-minted client credentials (preferred)
export OPUSDNS_API_ENDPOINT="http://api.opusdns.local"
export OPUSDNS_ORG_ID="organization_01jnh0v027fz2r0pcbavf9qtyy"
export OPUSDNS_CLIENT_SECRET="cs_xxx"      # from POST /v1/auth/client_credentials

# Option B: full username/password flow (mints a fresh api key per run)
export OPUSDNS_API_ENDPOINT="http://api.opusdns.local"
export OPUSDNS_ORG_ID="organization_01jnh0v027fz2r0pcbavf9qtyy"
export OPUSDNS_USERNAME="example_user"
export OPUSDNS_PASSWORD="securepassword123"
```

If you prefer to drive everything through `TF_VAR_*` (e.g. when committing Terraform configs that consume `var.opusdns_*` like [`examples/provider/provider.tf`](examples/provider/provider.tf)):

```sh
export TF_VAR_opusdns_api_endpoint="http://api.opusdns.local"
export TF_VAR_opusdns_org_id="organization_01jnh0v027fz2r0pcbavf9qtyy"
export TF_VAR_opusdns_client_secret="cs_xxx"
```

### 4. Run a smoke test against the example configs

The sibling repo [`test-opusdns-terraform`](../test-opusdns-terraform) contains ready-to-run Terraform configurations under `tests/`:

```
tests/
├── zone-basic/
├── zone-dnssec/
├── zone-datasources/
├── record-a/
├── record-multi-type/
├── record-update-ttl/
├── contact-minimal/
├── contact-full/
├── email-forward-basic/
├── email-forward-catchall/
├── domain-forward-https/
├── domain-forward-multi/
└── end-to-end/
```

Pick the smallest case to verify auth + a single CRUD round-trip:

```sh
cd ../test-opusdns-terraform/tests/zone-basic
terraform plan      # no `init` needed thanks to dev_overrides
terraform apply -auto-approve
terraform destroy -auto-approve
```

You should see Terraform skip provider download (with a warning that `dev_overrides` is in effect — that's expected), authenticate against your local API, and create/destroy a zone.

### 5. Tighter inner loop while editing provider code

```sh
# In one terminal, keep rebuilding on save:
ls internal/**/*.go provider.go | entr -r make install

# In another, re-run a focused test case:
cd ../test-opusdns-terraform/tests/record-a
terraform apply -auto-approve && terraform destroy -auto-approve
```

For richer debugging, set:

```sh
export TF_LOG=DEBUG                 # full Terraform + provider logs
export OPUSDNS_DEBUG=true           # SDK-level HTTP request/response logs
```

### 6. Quick provider-side checks

Before driving Terraform, you can sanity-check the Go code with:

```sh
make build      # compile only
make vet        # go vet
make test       # unit tests (none currently, but useful as you add them)
make fmt        # gofmt
```

There are no acceptance tests (`TF_ACC=1 go test ./...`) wired up yet — drive end-to-end coverage through the `test-opusdns-terraform` configs above.

## License

[Mozilla Public License 2.0](LICENSE)
