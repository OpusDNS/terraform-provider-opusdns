# Changelog

All notable changes to the OpusDNS Terraform provider are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.3.0] - 2026-10-23

This release moves the provider to [opusdns-go-client v1.3.2](https://github.com/OpusDNS/opusdns-go-client/releases/tag/v1.3.2), which matches the OpusDNS API (api-spec **1.243.0**, up from 1.147.0). The previous SDK decoded several responses into the wrong shape, so some data sources returned empty values without an error. Those data sources now read what the API actually sends. Their schemas follow the API, which removes and renames some attributes.

> [!WARNING]
> **This minor release contains breaking changes.** It removes and reshapes data-source attributes. A configuration that still references one of them fails to plan after the upgrade. Almost all of them had no effect before: the API does not send these fields or support these filters, so they were empty, zero or ignored. Please review the list below before upgrading.
>
> A `~> 1.0` version constraint picks up this release. Pin `~> 1.2.0` to stay on 1.2.x until your configuration is updated.

### ⚠️ Breaking changes

#### `opusdns_tld`

The data source now reads the TLD specification that the API returns. With the previous SDK, most of its attributes were empty.

| Removed | Use instead |
|---|---|
| `min_registration_period`, `max_registration_period` | `registration_periods`, a list of `{value, unit}` |
| `grace_period_days`, `redemption_period_days` | `grace_period`, `redemption_period`, ISO 8601 durations such as `P30D` |
| `restrictions` | `local_presence` (`required`, `eligible_countries`) |
| `phases[*].status` | `phases[*].supported` |
| `phases[*].requirements` | `phases[*].smd_required` |
| `available`, `registration_enabled`, `transfer_enabled`, `description`, `registry`, `pricing`, `nameserver_config.glue_records_required` | no replacement: the TLD specification has no such data, and no prices |

Also changed:
- `phases[*].name` holds the phase type.
- Unset `phases[*].start_date`, `end_date` and `allocation_method` are `null` instead of `""`.
- `contact_config[*].required` is derived from `min > 0`.

These attributes are kept and filled from the specification: `idn_supported`, `dnssec_supported`, `min_domain_length`, `max_domain_length`, `whois_server`, `rdap_server`, `nameserver_config` (`min`, `max`), `contact_config` and `type`. `renewal_periods` is new.

#### `opusdns_contact` and `opusdns_contacts`
- `updated_on` is removed. The API does not send it.
- `opusdns_contacts`: the `verified` filter is removed. The API does not support it.

#### `opusdns_domains`
- The `status` and `renewal_mode` filters are removed. `GET /v1/domains` has no such query parameters, so they never filtered anything.

### Behaviour changes

- **No more duplicate writes:** a POST, PATCH or DELETE request (creates, deletes and most updates) is no longer resent after a 5xx or a dropped connection, because the API may already have acted on it (a double registration or charge). Terraform reports the error instead. Run `terraform plan` to see the actual state before you apply again.
- **Requests the API asks to resend are retried:** a 409 or 503 that carries a `Retry-After` header is retried, for every request. The API answers `409 ERROR_DOMAIN_SUBSCRIPTION_PENDING` when a domain is deleted in the seconds before its registration is finalized, so destroying an `opusdns_domain` right after creating it waits and then succeeds.
- **Invalid IDs fail early:** a request with an empty resource ID fails before it is sent, with a clear error, instead of reaching a wrong API route.

### Fixes

- **`opusdns_tlds`** fills `idn_supported`, `dnssec_supported` and `min_`/`max_registration_period`. Before, these were always `false` or `0`, so `dnssec_supported = true` returned no TLDs. The periods count only whole years, and they are `null` when a TLD offers none.
- **`opusdns_domains_summary`** reads the API's current summary. Its attributes are unchanged.
  - `total_domains` includes sub-organizations.
  - `expiring_within_30_days` and `expiring_within_90_days` are `null` when the API reports no expiry counts.

### Deprecated
These are still in the schema, but they have no effect or are always `null`:
- `opusdns_tlds`: the `registration_enabled` filter, which is ignored with a warning (it used to match nothing), and `tlds[*].registration_enabled` / `tlds[*].transfer_enabled`. The API has no such flags. Use `available`.
- `opusdns_user`, `opusdns_users` and the `opusdns_user` resource: `status`, which the API no longer returns. It was deprecated in v1.2.1.

## [1.2.1] - 2026-07-16

### Changed

- `opusdns_user` (data source and resource) and `opusdns_users`: `status` is
  deprecated and always `null`, because the OpusDNS API no longer returns a
  user status.

## [1.2.0] - 2026-07-16

### Added

- **Resources** `opusdns_role` (custom organization roles) and
  `opusdns_vanity_nameserver_set`.
- **Data sources** `opusdns_role_permissions`, `opusdns_vanity_nameserver_set`
  and `opusdns_vanity_nameserver_sets`.
- `opusdns_domain`: `expected_price` to confirm a premium registration, and
  `claims_notice_acceptance_hash` to register a name that matches a TMCH
  trademark claim.
- `opusdns_zone` (resource and data source): `vanity_nameserver_set_id`.
- `opusdns_domains`: the `status_tags` / `status_tag_mode` filters and
  `status_tags` on each domain. `opusdns_domain`: `status_tags`, filled when
  `include_tags` is set.

### Changed

- **`opusdns_user_role_assignment`** manages the single role of a user
  (`role`, via `PUT /v1/users/{user_id}/role`) instead of a set of roles
  (`roles`). Configurations using `roles` must be updated.
- **`opusdns_roles`** returns a `roles` list (`label`, `name`, `description`,
  `built_in`, `permissions`) instead of `role_names` and `roles_json`.

## [1.1.0] - 2026-07-07

### Added

- **`opusdns_domain` transfer-in support** — set `transfer = true` with an
  `auth_code` to transfer an existing domain in from another registrar
  (`POST /v1/domains/transfer`) instead of registering a new one. `create_zone`
  is not supported for transfers, and `period_unit` must be `y` (the transfer
  API accepts a year count only). The transferred domain is then managed
  identically to a registered one. Backwards compatible: `transfer` defaults to
  `false`, so existing configurations are unaffected.

### Changed

- **Upgraded `opusdns-go-client` `v1.0.10` → `v1.1.1`.** This refreshes the SDK
  data models against the current OpusDNS API. As a result, several data-source
  schemas changed to match fields the API no longer returns (see **Removed**),
  and the `opusdns_domain_suggestions` output was reshaped (see below).
- **`opusdns_domain_suggestions`** now reflects the current suggest API: each
  suggestion exposes `available` (bool) and `premium` (bool) instead of `status`
  and `score`, and pricing is split into `price` and `renewal_price` objects with
  `amount`/`currency`/`period` fields (replacing the previous
  `register_price`/`renew_price`/`transfer_price` shape). The request filter
  `include_unavailable` was replaced by `premium`.
- **`opusdns_event` / `opusdns_events`** now expose `acknowledged_on` and no
  longer expose the removed audit fields (see **Removed**).
- Rewrote `RELEASING.md` to document the HCP Terraform Registry workflow.
- Corrected the license metadata in `README.md`.

### Removed

- **Fields dropped from data sources because the current API no longer returns
  them** (SDK `v1.1.1` model refresh). Configurations referencing these
  attributes must be updated:
  - `opusdns_contact` / `opusdns_contacts`: `verified`, `verified_on`
    (the `verified` *filter* on `opusdns_contacts` is retained).
  - `opusdns_event` / `opusdns_events`: `organization_id`, `user_id`,
    `ip_address`, `user_agent`, `source`.
  - `opusdns_events`: the `created_after` / `created_before` request filters.

### Notes

- The `opusdns_tld_portfolio` raw-HTTP workaround for the SDK's
  `TLDs.GetPortfolio` schema mismatch is retained pending verification that the
  `v1.1.1` `TLDPortfolio` model matches the bare-array API response.

## [1.0.1] - 2026-05-21

### Added

- Terraform Registry documentation: per-resource and per-data-source pages
  under `docs/`, plus how-to guides, generated via `tfplugindocs`.

## [1.0.0] - 2026-05-21

First public release. The provider targets the OpusDNS REST API and is built
on `terraform-plugin-framework`.

### Provider

- API-key authentication via the `api_key` provider attribute or the
  `OPUSDNS_API_KEY` environment variable.
- Configurable API endpoint via `endpoint` / `OPUSDNS_API_ENDPOINT`.
- Acceptance test suite (`TF_ACC=1`) wired into CI; releases are gated on
  acceptance tests against the `preview1` environment.
- GoReleaser pipeline publishing signed archives for the Terraform Registry.

### Resources

- **DNS** — `opusdns_zone`, `opusdns_record` (covers A, AAAA, ALIAS, CAA,
  CNAME, MX, NS, PTR, TXT, SRV, SSHFP, TLSA, DS, HTTPS, SVCB, and more).
- **Domains** — `opusdns_domain`, `opusdns_domain_dnssec`,
  `opusdns_domain_forward`, `opusdns_email_forward`, `opusdns_parking`.
- **Contacts** — `opusdns_contact`, `opusdns_contact_attribute_set` (TLD-scoped
  bundles of registry-specific attributes such as `DE_CONTACT_TYPE`,
  `NOMINET_CO_NO`, `SIDN_LEGAL_FORM`, `US_NEXUS_CATEGORY`),
  `opusdns_contact_attribute_link`.
- **Hosts & tags** — `opusdns_host`, `opusdns_tag`.
- **Users & access** — `opusdns_user`, `opusdns_user_role_assignment`.
- **Registrar connectivity** — `opusdns_registrar_credential` (supports
  `INTERNETX`, `MONIKER`, `DOMAIN_BESTELLSYSTEM`, `CENTRALNIC`, `OPUSDNS`,
  `ENOM`).

### Data sources

- **DNS** — `opusdns_zone`, `opusdns_zones`, `opusdns_record`, `opusdns_records`.
- **Domains** — `opusdns_domain`, `opusdns_domains`, `opusdns_domain_dnssec`,
  `opusdns_domain_availability`, `opusdns_domain_check` (premium pricing and
  TMCH claims keys), `opusdns_claims_notice` (single-key TMCH notice fetch),
  `opusdns_domain_forward`, `opusdns_domain_forwards`, `opusdns_email_forward`,
  `opusdns_email_forwards`, `opusdns_parking`, `opusdns_parkings`.
- **Contacts** — `opusdns_contact`, `opusdns_contacts`,
  `opusdns_contact_attribute_set`, `opusdns_contact_attribute_sets`.
- **Hosts & tags** — `opusdns_host`, `opusdns_tag`, `opusdns_tags`.
- **Organization** — `opusdns_organization`, `opusdns_organizations`,
  `opusdns_roles`.
- **Users** — `opusdns_user`, `opusdns_users`, `opusdns_user_role_assignment`,
  `opusdns_user_permissions`.
- **TLDs** — `opusdns_tld`, `opusdns_tlds`.
- **Registrar connectivity** — `opusdns_registrar_credential`,
  `opusdns_registrar_credentials`.

### Notes

- `opusdns_contact_attribute_link` is removed from state with a warning on
  `terraform destroy`; the API exposes no unlink endpoint, so the link
  persists until the contact or attribute set is deleted.
- A previous draft of the provider exposed `opusdns_organization` and
  `opusdns_organization_ip_restriction` resources. These were removed before
  release because the API does not offer lifecycle semantics that fit
  Terraform. Organization read-only data sources remain.

[Unreleased]: https://github.com/OpusDNS/terraform-provider-opusdns/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/OpusDNS/terraform-provider-opusdns/compare/v1.2.1...v1.3.0
[1.2.1]: https://github.com/OpusDNS/terraform-provider-opusdns/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/OpusDNS/terraform-provider-opusdns/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/OpusDNS/terraform-provider-opusdns/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/OpusDNS/terraform-provider-opusdns/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/OpusDNS/terraform-provider-opusdns/releases/tag/v1.0.0
All notable changes to this provider are documented in this file.

## Unreleased

Changes on branch `remove-organization-ip-restriction-resource` (since `v1.0.4`).

### Removed

- **Resource `opusdns_organization_ip_restriction`** (commit `5caacc7`). Organization IP restrictions are managed exclusively through the OpusDNS web interface.
- **Data sources `opusdns_organization_ip_restriction` and `opusdns_organization_ip_restrictions`** (commit `6080ad4`). Same rationale as the resource removal — there is no Terraform-side use case for reading these either.
- Associated documentation under `docs/resources/` and `docs/data-sources/`, example modules under `examples/`, and template files under `templates/`.

### Added

- **`EXCLUDED_API_FEATURES.md`** (commit `2a40f77`) — reference file listing API surface area that must not be implemented as Terraform resources or data sources. Future API-sync audits must consult this file before adding new coverage. Seeded with the organization IP restrictions entry.

## v1.0.4 — 2026-06-03

### Fixed

- `opusdns_zone` resource: refresh full state immediately after `Create` via `GetZoneWithOptions(..., include=tags)`. The API's `CreateZone` response omits `zone_id`, `created_on`, `updated_on`, and `tags`, which previously broke `ImportStateVerify` parity in `TestAccZoneResource_basic`.

### Changed

- Skip `TestAccRecordResource_NS` pending an upstream API fix. `PATCH /v1/dns/{zone}/rrsets` for an NS rrset at a delegated subdomain returns 204 but the rrset is silently absent from the subsequent `GET`, causing a perpetual refresh-plan drift. Full repro lives in `docs/bugs/api-delegated-ns-dropped.md`.

## v1.0.2 — 2026-06-03

### Added

- **New data source `opusdns_report`** — retrieve a single report by id.
- **New data source `opusdns_reports`** — list reports with server-side filtering.
- **New data source `opusdns_tld_portfolio`** — list the organization's TLD portfolio (`GET /v1/tlds/portfolio`). Implemented against the raw HTTP client because the SDK's `TLDs.GetPortfolio` expects a wrapper object while the API returns a bare array.
- **New data source `opusdns_zones_summary`** — DNS zone counts/summary (`DNS.GetSummary`).
- **New data source `opusdns_domains_summary`** — domain counts/summary (`Domains.GetSummary`).
- **New data source `opusdns_domain_suggestions`** — availability suggestions (`Availability.GetSuggestions`).
- **New data source `opusdns_event`** — fetch a single event by id; exposes the dynamic payload as `event_data_json`.
- **New data source `opusdns_events`** — list events with server-side filtering and pagination.
- **`include_tags` support** — `opusdns_zone`, `opusdns_zones`, `opusdns_domain`, and `opusdns_domains` data sources accept `include_tags = true` and surface a computed `tags` list (`tag_id`, `label`, `color`). The `opusdns_zone` resource now always requests `include=tags` during refresh.
- **Server-side filters on existing list data sources**:
  - `opusdns_contacts`: `tag_ids`, `tag_mode`, `created_after`, `created_before`.
  - `opusdns_zones`: `search`, `name`, `suffix`, `dnssec_status`, `tag_ids`, `tag_mode`, `created_after`, `created_before`, `updated_after`, `updated_before`.
  - `opusdns_domains`: `tag_ids`, `tag_mode`, `include_tags`, `created_after`/`before`, `updated_after`/`before`, `expires_after`/`before`, `expires_in_30/60/90_days`, `registered_after`/`before`, `registry_statuses_in`.
- **Zone metadata** — `opusdns_zone` (resource and data source) and `opusdns_zones` now expose `zone_id`, `created_on`, and `updated_on`.
- **Examples** for the new resource and data sources under `examples/`.
- **Documentation** for all new and updated resources/data sources under `docs/` (with matching templates under `templates/`).

### Changed

- `opusdns_zone` resource refresh now uses `GetZoneWithOptions(..., include=tags)` so tags are always present in state.
- Domain/zone data sources route through `GetDomainWithOptions` / `GetZoneWithOptions` when `include_tags = true`.
- `.gitignore` updated.

### Fixed

- Integrated compatibility changes for the recent OpusDNS API release (see commit `fa71101`).

### Notes

- All additions are backwards compatible: new fields are `Optional`/`Computed` and existing configurations continue to plan/apply without changes.
- `opusdns_tld_portfolio` is implemented via the raw HTTP client as a workaround for an SDK/API schema mismatch in `opusdns-go-client@v1.0.10`'s `TLDs.GetPortfolio`. Track upstream for the eventual SDK fix.
