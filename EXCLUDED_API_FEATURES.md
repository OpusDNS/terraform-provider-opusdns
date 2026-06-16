# Excluded API Features

This file lists OpusDNS API endpoints, resources, and features that are
**deliberately excluded** from the Terraform provider. They will not be
implemented as resources or data sources, regardless of SDK or API
coverage.

## How to use this file

**Before adding any new resource or data source** — whether as part of a
scheduled API sync, a feature request, or a one-off audit — check this
list first. If the feature appears here, do not implement it; reference
this file (and the linked rationale) in any discussion of why.

When the rationale for an exclusion changes (e.g. the API exposes a
real management endpoint where before there was only a UI-only flow),
remove the entry from this file in the same commit that re-introduces
the resource/data source.

## Exclusions

### Organization IP restrictions

- **Excluded surface area**: any resource or data source named
  `opusdns_organization_ip_restriction` or
  `opusdns_organization_ip_restrictions`.
- **API endpoints**: `GET/POST/PATCH/DELETE /v1/organization/ip-restrictions`
  and `GET /v1/organization/ip-restrictions/{id}` (any future variants).
- **SDK references**: `Organizations.ListIPRestrictions`,
  `Organizations.GetIPRestriction`,
  `Organizations.CreateIPRestriction`,
  `Organizations.UpdateIPRestriction`,
  `Organizations.DeleteIPRestriction`,
  `models.IPRestriction`, and any future helpers added under the same
  prefix in `opusdns-go-client`.
- **Rationale**: organization IP restrictions are managed exclusively
  through the OpusDNS web interface. Surfacing them in Terraform would
  let operators create configurations that the web UI also expects to
  own, and there is no use case for reading them from Terraform either
  (they have no dependency relationship with any other provider
  resource).
- **Decision date / commits**:
  - 2026-06-03 — initial removal of the resource
    (`5caacc7` — *Remove opusdns_organization_ip_restriction resource*).
  - 2026-06-03 — removal of the matching data sources
    (`6080ad4` — *Remove opusdns_organization_ip_restriction(s) data
    sources*).

## Adding a new exclusion

Use the same structure as the entries above. Required fields:

- **Excluded surface area** — exact resource/data-source type names.
- **API endpoints** — every route covered by the exclusion.
- **SDK references** — every Go SDK identifier covered by the exclusion.
- **Rationale** — why the exclusion exists (one short paragraph).
- **Decision date / commits** — short-SHA + one-line subject of the
  commit that enforced the exclusion (or the discussion link if the
  decision predates the commit).
