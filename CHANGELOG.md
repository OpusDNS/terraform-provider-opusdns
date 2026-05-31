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
