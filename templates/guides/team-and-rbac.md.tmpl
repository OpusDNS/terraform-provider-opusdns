---
page_title: "Team & RBAC Management"
subcategory: ""
description: |-
  Manage users, roles, organizations, and tags with the OpusDNS Terraform provider. Implement fine-grained RBAC for your team.
---

# Team & RBAC Management

This guide covers user management, role-based access control (RBAC), organization structure, and resource tagging — everything you need to run a secure, well-organized DNS operation.

## Users

### Creating Team Members

```terraform
resource "opusdns_user" "alice" {
  username   = "alice"
  email      = "alice@mycompany.com"
  first_name = "Alice"
  last_name  = "Anderson"
  phone      = "+12125550100"
  locale     = "en-US"
}

resource "opusdns_user" "bob" {
  username   = "bob"
  email      = "bob@mycompany.com"
  first_name = "Bob"
  last_name  = "Builder"
  phone      = "+442071234567"
  locale     = "en-GB"
}
```

-> **Note:** `username` and `email` are immutable. Changing either will recreate the user.

### Reading User Information

```terraform
# Get the current authenticated user
data "opusdns_user" "me" {
  me = true
}

# List all users in the organization
data "opusdns_users" "team" {}

output "team_members" {
  value = [for u in data.opusdns_users.team.users : "${u.first_name} ${u.last_name} (${u.username})"]
}
```

## Roles & Permissions

OpusDNS provides 16 built-in roles for fine-grained access control. Assign roles to users with the `opusdns_user_role_assignment` resource.

### Available Roles

| Role | Description |
|------|-------------|
| `admin` | Full administrative access |
| `api_admin` | Manage API credentials |
| `billing_manager` | View and manage billing |
| `chat_manager` | Manage support chat |
| `cms_content_editor` | Edit CMS content |
| `contact_manager` | Create and manage contacts |
| `domain_forward_manager` | Manage domain forwards |
| `domain_manager` | Register and manage domains |
| `email_forward_manager` | Manage email forwards |
| `events_manager` | View and manage events |
| `host_manager` | Manage host (glue) objects |
| `member` | Basic read access |
| `organization_manager` | Manage sub-organizations |
| `product_manager` | Manage products |
| `registrar_credential_manager` | Manage registrar credentials |
| `reseller_manager` | Manage reseller operations |

### Assigning Roles

```terraform
# DevOps engineer: full DNS + domain control
resource "opusdns_user_role_assignment" "alice_devops" {
  user_id = opusdns_user.alice.id

  roles = [
    "member",
    "domain_manager",
    "contact_manager",
    "host_manager",
    "email_forward_manager",
    "domain_forward_manager",
  ]
}

# Developer: read-only member access
resource "opusdns_user_role_assignment" "bob_dev" {
  user_id = opusdns_user.bob.id

  roles = [
    "member",
  ]
}

# Platform lead: almost everything
resource "opusdns_user_role_assignment" "carol_lead" {
  user_id = opusdns_user.carol.id

  roles = [
    "member",
    "domain_manager",
    "contact_manager",
    "host_manager",
    "email_forward_manager",
    "domain_forward_manager",
    "api_admin",
    "organization_manager",
  ]
}
```

### Role Templates with Locals

Define role presets for consistency:

```terraform
locals {
  role_presets = {
    viewer = ["member"]
    dns_operator = [
      "member",
      "domain_manager",
      "host_manager",
      "email_forward_manager",
      "domain_forward_manager",
    ]
    domain_admin = [
      "member",
      "domain_manager",
      "contact_manager",
      "host_manager",
      "registrar_credential_manager",
    ]
    full_admin = [
      "admin",
      "member",
      "domain_manager",
      "contact_manager",
      "host_manager",
      "email_forward_manager",
      "domain_forward_manager",
      "api_admin",
      "billing_manager",
      "organization_manager",
      "registrar_credential_manager",
    ]
  }
}

variable "team" {
  default = {
    alice = { preset = "dns_operator" }
    bob   = { preset = "viewer" }
    carol = { preset = "full_admin" }
  }
}

resource "opusdns_user_role_assignment" "team" {
  for_each = var.team

  user_id = opusdns_user.members[each.key].id
  roles   = local.role_presets[each.value.preset]
}
```

### Checking Permissions

Verify what a user can actually do:

```terraform
data "opusdns_user_permissions" "alice" {
  user_id = opusdns_user.alice.id
}

output "alice_can_do" {
  value = data.opusdns_user_permissions.alice.permissions
}
```

### Listing Available Roles

```terraform
data "opusdns_roles" "available" {}

output "all_roles" {
  value = data.opusdns_roles.available.role_names
}
```

## Organizations

Organizations provide multi-tenancy. View your org and child organizations:

```terraform
# Get the current organization
data "opusdns_organization" "current" {
  me = true
}

output "org_name" {
  value = data.opusdns_organization.current.name
}

# List child organizations
data "opusdns_organizations" "children" {}

output "child_orgs" {
  value = [for o in data.opusdns_organizations.children.organizations : o.name]
}
```

## Tags

Tags help categorize and organize your resources. They support three types: `DOMAIN`, `CONTACT`, and `ZONE`.

### Creating Tags

```terraform
resource "opusdns_tag" "production" {
  label       = "production"
  type        = "DOMAIN"
  color       = "color-1"
  description = "Production-tier domains — handle with care!"
}

resource "opusdns_tag" "staging" {
  label       = "staging"
  type        = "DOMAIN"
  color       = "color-4"
  description = "Staging and test domains."
}

resource "opusdns_tag" "client_a" {
  label       = "client-a"
  type        = "DOMAIN"
  color       = "color-7"
  description = "Domains managed for Client A."
}
```

-> **Note:** The `type` field is immutable — changing it forces replacement. `label`, `color`, and `description` are updatable in place.

### Querying Tags

```terraform
# Get all domain tags
data "opusdns_tags" "domain_tags" {
  tag_types = ["DOMAIN"]
}

# Search for production tags
data "opusdns_tags" "prod" {
  search = "prod"
}

output "tag_labels" {
  value = [for t in data.opusdns_tags.domain_tags.tags : t.label]
}
```

## Registrar Credentials

For multi-registrar setups (OpusDNS Connect), manage credentials declaratively:

```terraform
resource "opusdns_registrar_credential" "primary" {
  name      = "InternetX Production"
  registrar = "INTERNETX"

  credentials = {
    username = var.internetx_username
    password = var.internetx_password
  }
}

resource "opusdns_registrar_credential" "backup" {
  name      = "CentralNic Backup"
  registrar = "CENTRALNIC"

  credentials = {
    api_key = var.centralnic_api_key
  }
}
```

Supported registrars: `INTERNETX`, `MONIKER`, `DOMAIN_BESTELLSYSTEM`, `CENTRALNIC`, `OPUSDNS`, `ENOM`.

~> **Security:** Credential values are write-only. The API never returns them, so Terraform cannot detect drift. Re-supply credentials in config after importing.

### Listing Registrar Credentials

```terraform
data "opusdns_registrar_credentials" "all" {}

# Filter by registrar
data "opusdns_registrar_credentials" "internetx_only" {
  registrar = "INTERNETX"
}

output "credential_names" {
  value = [for c in data.opusdns_registrar_credentials.all.registrar_credentials : c.name]
}
```

## Putting It All Together

Here's a complete team setup for a growing startup:

```terraform
# --- Users ---
locals {
  team_members = {
    alice = {
      email      = "alice@startup.io"
      first_name = "Alice"
      last_name  = "Anderson"
      role       = "full_admin"
    }
    bob = {
      email      = "bob@startup.io"
      first_name = "Bob"
      last_name  = "Builder"
      role       = "dns_operator"
    }
    carol = {
      email      = "carol@startup.io"
      first_name = "Carol"
      last_name  = "Clark"
      role       = "viewer"
    }
  }
}

resource "opusdns_user" "team" {
  for_each = local.team_members

  username   = each.key
  email      = each.value.email
  first_name = each.value.first_name
  last_name  = each.value.last_name
}

resource "opusdns_user_role_assignment" "team" {
  for_each = local.team_members

  user_id = opusdns_user.team[each.key].id
  roles   = local.role_presets[each.value.role]
}

# --- Tags for organization ---
resource "opusdns_tag" "environments" {
  for_each = toset(["production", "staging", "development"])

  label       = each.value
  type        = "DOMAIN"
  color       = each.value == "production" ? "color-1" : each.value == "staging" ? "color-4" : "color-7"
  description = "${title(each.value)} environment domains."
}
```

## Import

```shell
terraform import opusdns_user.alice <user_id>
terraform import opusdns_user_role_assignment.alice <user_id>
terraform import opusdns_tag.production <tag_id>
terraform import opusdns_registrar_credential.primary <registrar_credential_id>
```

## What's Next?

- **[Getting Started](/docs/guides/getting-started)** — New to the provider? Start here
- **[Domain Registration](/docs/guides/domain-registration)** — Register domains your team will manage
- **[DNS Zones & Records](/docs/guides/dns-zones-and-records)** — Set up DNS for your domains
