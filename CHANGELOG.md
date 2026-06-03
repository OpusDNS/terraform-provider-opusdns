# Changelog

All notable changes to this provider are documented in this file.

## Unreleased

Changes since commit `d52165a` ("Merge pull request #52 from OpusDNS/update-readme-fix-license").

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
