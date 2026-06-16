# Changelog

All notable changes to the OpusDNS Terraform provider are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Rewrote `RELEASING.md` to document the HCP Terraform Registry workflow.
- Corrected the license metadata in `README.md`.

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

[Unreleased]: https://github.com/OpusDNS/terraform-provider-opusdns/compare/v1.0.1...HEAD
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
