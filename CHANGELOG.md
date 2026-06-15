# Changelog

All notable changes to this provider are documented in this file.

## Unreleased

Changes on branch `remove-organization-ip-restriction-resource` (since `v1.0.4`).

### Removed

- **Resource `opusdns_organization_ip_restriction`** (commit `5caacc7`). Organization IP restrictions are managed exclusively through the OpusDNS web interface.
- **Data sources `opusdns_organization_ip_restriction` and `opusdns_organization_ip_restrictions`** (commit `6080ad4`). Same rationale as the resource removal — there is no Terraform-side use case for reading these either.
- Associated documentation under `docs/resources/` and `docs/data-sources/`, example modules under `examples/`, and template files under `templates/`.

### Added

- **New data source `opusdns_organization_attributes`** — read organization-level key/value attributes (`GET /v1/organizations/attributes` or `.../{organization_id}/attributes`). Either set `organization_id`, set `me = true`, or leave both unset to hit the caller-org endpoint. Optional `keys` filter narrows the response. Arbitrary JSON values are surfaced as a string in `value_json` (decode with `jsondecode()`).
- **New data source `opusdns_organization_transactions`** — list billing transactions with server-side filters (product_type, action, status, created window) and pagination.
- **New data source `opusdns_organization_transaction`** — fetch a single billing transaction by id.
- **New data source `opusdns_organization_invoices`** — list invoices from `GET /v1/organizations/{organization_id}/billing/invoices` with pagination. Money fields are surfaced as decimal strings to preserve precision.
- **New data source `opusdns_organization_pricing`** — read product prices from `GET /v1/organizations/{organization_id}/pricing/product-type/{product_type}`. Optional `product_action` / `product_class` query parameters narrow the response. Money fields are surfaced as decimal strings.
- **`opusdns-go-client` v1.0.10 SDK drift documented** in `docs/bugs/sdk-organizations-billing-pricing-drift.md`. The attributes / invoices / pricing SDK helpers build wrong paths and/or expect wrong response shapes; the new data sources bypass them via the raw HTTP client (same pattern used for `opusdns_tld_portfolio`).
- **New data source `opusdns_request_history`** — list API request history entries (`GET /v1/archive/request-history`) with server-side filters (method/path/status-code range/duration range/actor/time window) and pagination.
- **New data source `opusdns_object_logs`** — list object change-log entries (`GET /v1/archive/object-logs`) with server-side filters (object_type/object_id/action/user/time window) and pagination. The dynamic `changes` payload is surfaced as JSON via `changes_json` (decode with `jsondecode()`).
- **New data source `opusdns_email_forward_logs`** — list email-forward delivery logs. Set either `email_forward_id` (reads `GET /v1/archive/email-forward-logs/{id}`) or `email_forward_alias_id` (reads `GET /v1/archive/email-forward-logs/aliases/{id}`). Exactly one must be provided.
- **`EXCLUDED_API_FEATURES.md`** (commit `2a40f77`) — reference file listing API surface area that must not be implemented as Terraform resources or data sources. Future API-sync audits must consult this file before adding new coverage. Seeded with the organization IP restrictions entry.
- Generated documentation under `docs/data-sources/` for the new archive data sources and for several v1.0.2 data sources that previously lacked rendered docs (`event`, `events`, `tld_portfolio`, `zones_summary`, `domains_summary`, `domain_suggestions`).

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
