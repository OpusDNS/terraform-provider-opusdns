# Plan: Transfer-in support on `opusdns_domain`

**Status:** Proposed (not yet implemented)
**Author:** API/provider audit follow-up
**Scope:** Domain CRUD — add domain transfer-in as a create path on the existing
`opusdns_domain` resource.
**SDK:** `github.com/opusdns/opusdns-go-client@v1.0.10` (no bump required — all
required methods already exist).

---

## Summary

Today `opusdns_domain` can only **register** a new domain (`POST /v1/domains`
via `Domains.CreateDomain`). The OpusDNS API also supports **transferring in** an
existing domain from another registrar (`POST /v1/domains/transfer` via
`Domains.TransferDomain`), which the provider does not expose. This plan adds
transfer-in as an **alternate create path on the existing `opusdns_domain`
resource**, gated by a new `transfer` boolean.

A transferred-in domain becomes an ordinary managed domain: its Read, Update,
Delete, and Import behaviour is identical to a registered domain. The **only**
divergence is the create-time API call. Modelling it as a create-path (rather
than a separate `opusdns_domain_transfer` resource) keeps a single coherent
lifecycle and avoids duplicating Read/Update/Delete/Import.

---

## Background / evidence

- **SDK method exists** (`opusdns/service_domains.go`):

  ```go
  func (s *DomainsService) TransferDomain(ctx context.Context, req *models.DomainTransferRequest) (*models.Domain, error)
  // POST /v1/domains/transfer
  ```

- **Request model** (`models/domains.go`):

  ```go
  type DomainTransferRequest struct {
      Name        string                                  `json:"name"`
      AuthCode    string                                  `json:"auth_code"`
      Contacts    map[DomainContactType][]ContactHandle   `json:"contacts,omitempty"`
      RenewalMode RenewalMode                             `json:"renewal_mode"`
      Nameservers []Nameserver                            `json:"nameservers,omitempty"`
      Period      int                                     `json:"period,omitempty"` // years only
  }
  ```

- **Diff vs. `DomainCreateRequest`:**
  - `auth_code` is a **required, non-pointer string** on transfer (vs. optional
    `*string` on create).
  - `Period` is a bare **int (years)** — there is **no period unit** and **no
    `create_zone`** on the transfer request.
  - `Name`, `Contacts`, `RenewalMode`, `Nameservers` are the same shape, so the
    existing `contactsMapToAPI` / `nameserversListToAPI` helpers are reused
    as-is.

- **Returns `*models.Domain`** — the same type the create path already feeds to
  `populateDomainResourceModel`, so post-create state hydration and the existing
  transfer-lock reconciliation are unchanged.

- **Not excluded:** `EXCLUDED_API_FEATURES.md` only bars organization IP
  restrictions; nothing here is excluded.

---

## Design decisions (locked)

1. **Create-path on `opusdns_domain`** (not a separate resource). Confirmed.
2. **Acceptance test ships `t.Skip`-gated.** A real transfer needs a domain at a
   losing registrar plus a valid auth code, which CI cannot provision
   on-demand; we follow the repo precedent (`TestAccRecordResource_NS` is
   skipped pending external conditions). Unit tests cover the new pure logic and
   run in `make test`.

---

## Schema changes (`internal/provider/resource_domain.go`)

### New attribute

Add to `DomainResourceModel`:

```go
Transfer types.Bool `tfsdk:"transfer"`
```

Add to the schema `Attributes` map:

```go
"transfer": schema.BoolAttribute{
    Optional:            true,
    Computed:            true,
    Default:             booldefault.StaticBool(false),
    MarkdownDescription: "When `true`, the domain is transferred in from another " +
        "registrar via `POST /v1/domains/transfer` instead of being registered. " +
        "Requires `auth_code`. `create_zone` is not supported for transfers, and " +
        "`period_unit` must be `y` (the transfer API accepts a year count only). " +
        "Forces replacement.",
    PlanModifiers: []planmodifier.Bool{
        boolplanmodifier.RequiresReplace(),
        boolplanmodifier.UseStateForUnknown(),
    },
},
```

### Attribute-interaction rules

The framework cannot express "required-when" directly, so enforce these via
`ConfigValidators` (preferred) or an early `Create`-time validation:

- `transfer = true` ⇒ `auth_code` must be set and non-empty.
- `transfer = true` ⇒ `create_zone` must be unset/`false` (transfer request has
  no `create_zone`; erroring is clearer than silently ignoring).
- `transfer = true` ⇒ `period_unit` must be `"y"` (transfer `Period` is a bare
  year count). If `period_unit` is `m`/`d` with `transfer = true`, error.

Implement `resource.ResourceWithConfigValidators` returning custom validators, or
a single `validateDomainConfig(model) diag.Diagnostics` pure function called at
the top of `Create`. The pure-function approach is preferred for unit-testability
(see Tests).

### Doc-string correction

The resource `MarkdownDescription` currently states:

> "Premium pricing confirmation, TMCH claims acceptance, **transfer-in**, restore,
> and DNSSEC are not modeled here…"

Remove "transfer-in" from that exclusion sentence once implemented.

---

## Create logic (`internal/provider/resource_domain.go`)

Branch in `Create` after config validation:

```go
// (pseudocode)
if data.Transfer.ValueBool() {
    transferReq, diags := buildDomainTransferRequest(ctx, data)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }

    domain, err := r.client.Domains.TransferDomain(ctx, transferReq)
    if err != nil {
        resp.Diagnostics.AddError("Error transferring domain", formatAPIError(err))
        return
    }
    // converge: same transfer-lock reconciliation + populate as register path
} else {
    // existing CreateDomain path, unchanged
}
```

Extract a pure helper mirroring the existing inline create-request construction:

```go
func buildDomainTransferRequest(ctx context.Context, data DomainResourceModel) (*models.DomainTransferRequest, diag.Diagnostics)
```

It reuses `contactsMapToAPI` and `nameserversListToAPI`, maps `period_value` →
`Period` (int years), and sets `Name`, `AuthCode`, `RenewalMode`.

**Transfer-lock note:** the existing post-create reconciliation that toggles
`clientTransferProhibited` to match `transfer_lock` applies unchanged. (A
just-transferred domain is typically unlocked; the existing logic already
reconciles whatever the registry returns to the desired state.)

Keep `Create` short by delegating to `buildDomainTransferRequest` and a small
`buildDomainCreateRequest` (optional refactor of the current inline block for
symmetry).

---

## Read / Update / Delete

**No changes.** A transferred-in domain is read, updated, and deleted exactly
like a registered one (`GetDomain` / `UpdateDomain` / `DeleteDomain` keyed on
`domain_id`). `create_zone`-based side-effect zone cleanup on destroy does not
apply to transfers (transfers can't set `create_zone`), so no Delete change is
needed.

---

## Import (`internal/provider/resource_domain.go`)

In `ImportState`, default the new input the same way `create_zone`/`auth_code`
are defaulted today (these inputs aren't recoverable from a `GET`):

```go
data.Transfer = types.BoolValue(false)
```

Post-import, `transfer` is irrelevant to ongoing management, so defaulting to
`false` is correct and matches existing import behaviour.

---

## Tests (`internal/provider/resource_domain_test.go`)

### Acceptance test (skip-gated)

Add `TestAccDomainResource_transfer`, following the existing structure of
`TestAccDomainResource_basic`, but `t.Skip(...)` by default with a message
explaining it requires a transferable domain + valid auth code (mirrors the
`TestAccRecordResource_NS` skip convention). Add a config builder:

```go
func testAccDomainResourceConfigTransfer(domainName, contactKey, authCode string) string
```

The config sets `transfer = true` and `auth_code = <authCode>` and omits
`create_zone`.

### Unit tests (run in `make test`, no API)

Cover the new pure logic:

- `Test_validateDomainConfig` — table-driven:
  - `transfer=true` + empty `auth_code` ⇒ error.
  - `transfer=true` + `create_zone=true` ⇒ error.
  - `transfer=true` + `period_unit="m"` ⇒ error.
  - `transfer=true` + valid (`auth_code` set, `period_unit="y"`,
    `create_zone=false`) ⇒ no error.
  - `transfer=false` (register) ⇒ no transfer-specific errors.
- `Test_buildDomainTransferRequest` — verifies field mapping (name, auth_code,
  contacts, nameservers, renewal_mode, period years) from a representative model.

---

## Docs & examples

The repo generates `docs/` from `templates/` + schema via `tfplugindocs`
(`make generate-docs`).

1. **Example:** add a transfer block to
   `examples/resources/opusdns_domain/resource.tf` (or a clearly-commented second
   resource in that file) showing `transfer = true` + `auth_code`.
2. **Regenerate docs:** run `make generate-docs` to refresh
   `docs/resources/domain.md` from `templates/resources/domain.md.tmpl` and the
   updated schema. Do **not** hand-edit `docs/resources/domain.md`.
3. **README:** update the `opusdns_domain` section to mention transfer-in
   (inputs: `transfer`, required `auth_code`; constraints: no `create_zone`,
   `period_unit = "y"`).
4. **CHANGELOG:** add an `[Unreleased] → Added` entry:
   *"`opusdns_domain` now supports transfer-in via `transfer = true` +
   `auth_code` (`POST /v1/domains/transfer`)."*

---

## Validation / sign-off

Run before opening a PR (acceptance tests remain CI-gated per repo convention):

```sh
make fmt
make vet
go build ./...
make test          # unit tests (validators + builder) + build
make generate-docs # then verify docs/resources/domain.md diff is intended
```

`make testacc` (including the new skip-gated transfer test) runs in CI; the
transfer case stays skipped until a sandbox transfer fixture exists.

---

## Files touched

| File | Change |
|---|---|
| `internal/provider/resource_domain.go` | Add `transfer` attr, config validators (or `validateDomainConfig`), `buildDomainTransferRequest`, branch in `Create`, default `transfer` in `ImportState`, correct doc-string. |
| `internal/provider/resource_domain_test.go` | Add skip-gated `TestAccDomainResource_transfer` + config builder; add unit tests for validators and request builder. |
| `examples/resources/opusdns_domain/resource.tf` | Add transfer-in example. |
| `docs/resources/domain.md` | Regenerated via `make generate-docs`. |
| `README.md` | Document transfer-in on `opusdns_domain`. |
| `CHANGELOG.md` | `[Unreleased] → Added` entry. |

---

## Out of scope (intentionally)

These domain-lifecycle operations exist in the SDK but are **imperative** and a
poor fit for declarative Terraform; they are deliberately excluded from this
plan and recommended to be documented as out-of-scope rather than implemented:

- **Renew** (`Domains.RenewDomain`) — Terraform already manages auto-renew
  declaratively via `renewal_mode`.
- **Restore** (`Domains.RestoreDomain`) — recovery action on a redemption-state
  domain, not steady-state config.
- **TLD-specific domain operations** (withdraw/transit/registry-specific
  auth-code flows) — no SDK helpers; imperative.

A separate, optional follow-up could add a read-only
`data.opusdns_contact_verification` (via `Contacts.GetVerificationStatus`), but
that is unrelated to this plan.
