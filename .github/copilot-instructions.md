# Copilot Instructions

## Build, test, and lint

```sh
make build          # go build ./...
make test           # go test ./... -v -count=1 -timeout 120s
make vet            # go vet ./...
make fmt            # gofmt -s -w .
make generate-docs  # regenerate Terraform Registry docs from schemas + templates
golangci-lint run   # lint (config in .golangci.yml)

# Run a single test
go test ./internal/provider/... -run TestFunctionName -v

# Check Terraform example formatting
terraform fmt -check -recursive -diff examples/
```

Active linters: `errcheck`, `govet`, `ineffassign`, `misspell`, `staticcheck`, `unconvert`, `unused`. Formatters: `gofmt`, `goimports`.

## Architecture

All provider code lives in `internal/provider/`. The module uses **Terraform Plugin Framework v1** (`hashicorp/terraform-plugin-framework`) and the **opusdns-go-client** SDK (`github.com/opusdns/opusdns-go-client`).

- **Resources** → `resource_<name>.go` (constructor: `New<Name>Resource()`)
- **Data sources** → `datasource_<name>.go` (constructor: `New<Name>DataSource()`)
- **Complex resource helpers** → `<name>_helpers.go` (e.g. `contact_attribute_set_helpers.go`)
- **Custom types** → `fqdn_type.go`, `phone_type.go`
- **Shared helpers** → `helpers.go`

Every resource/data source is registered in `provider.go` inside `Resources()` / `DataSources()`.

The provider authenticates with a single mode:
1. `api_key` (or `OPUSDNS_API_KEY`) → direct API key authentication via the SDK's native `X-Api-Key` header support

## Key conventions

### FQDN / trailing-dot normalization
The OpusDNS API returns domain names in FQDN form with a trailing dot (e.g. `example.com.`), but users write and expect the dot-less form. This is handled in two layers:
- **`fqdnType` / `fqdnValue`** (`fqdn_type.go`): a custom Terraform string type with `StringSemanticEquals` that treats `example.com` and `example.com.` as equal. Use `fqdnType{}` / `fqdnValue{}` for zone name and id attributes.
- **`trimTrailingDot` / `normalizeRData`** (`helpers.go`): strip trailing dots from API responses before writing to state for non-FQDN-typed attributes. `normalizeRData` is aware of record types (CNAME, DNAME, MX, NS, PTR, SRV) that embed FQDNs.

### Phone number normalization
`phoneType` / `phoneValue` (`phone_type.go`) handles semantic equality for phone/fax fields that the API may reformat (e.g. `+1.2125551234` ↔ `+1 212-555-1234`).

### Resource / data source structure
Each file follows this pattern:
```go
// Compile-time interface check
var _ resource.Resource = &XxxResource{}
var _ resource.ResourceWithImportState = &XxxResource{}

type XxxResource struct{ client *opusdns.Client }
type XxxResourceModel struct { /* tfsdk tags */ }

func NewXxxResource() resource.Resource { return &XxxResource{} }
// Metadata, Schema, Configure, Create, Read, Update, Delete, ImportState
```

`Configure` asserts `req.ProviderData.(*opusdns.Client)` and stores it on the struct.

### Error handling
- **404 detection**: `isNotFound(err)` (wraps `opusdns.ErrNotFound`). In `Read`, call `resp.State.RemoveResource(ctx)` on 404 instead of returning an error.
- **Error formatting**: always use `formatAPIError(err)` — it surfaces HTTP status, `request_id`, `details`, and raw body from `*opusdns.APIError`.

### Optional field helpers (`helpers.go`)
| Helper | Use |
|---|---|
| `addOptionalString(body, key, v)` | Add a `types.String` to a `map[string]interface{}` only if set |
| `optionalStringPtr(v)` | Convert `types.String` → `*string` (nil when null/unknown) |
| `stringPtrToValue(p)` | Convert `*string` → `types.String` (null when nil) |
| `timePtrToValue(t)` | Convert `*time.Time` → RFC3339 `types.String` (null when nil) |
| `mapToStringMap(ctx, m)` | Convert `types.Map` → `map[string]string` |

### SDK workarounds
Some SDK helpers are missing or broken; use the raw HTTP wrappers in `helpers.go`:
- `rawCreateOrganization` / `rawDeleteOrganization` — SDK has no create/delete org helpers
- `rawListEmailForwardsByZone` — SDK decodes the wrong shape; this wrapper tries the actual API wrapper struct first, then falls back to a bare list

When an SDK method is missing, issue calls via `c.HTTPClient().BuildPath(...)` + `c.HTTPClient().Get/Post/Delete(...)` + `c.HTTPClient().DecodeResponse(...)`.

### Record resource
`opusdns_record` represents a full **RRSet** (all records sharing the same name + type in a zone). The `id` is formatted as `zone_name/name/type`. The `records` attribute is a list of rdata strings.

## Documentation (Terraform Registry)

Registry docs are auto-generated with [`tfplugindocs`](https://github.com/hashicorp/terraform-plugin-docs) via `make generate-docs`. The output lives in `docs/` and **must not be hand-edited** — edit the sources instead:

| What to edit | Location |
|---|---|
| Provider index page | `templates/index.md.tmpl` |
| Resource / data source docs | `templates/resources/<name>.md.tmpl` or `templates/data-sources/<name>.md.tmpl` |
| Guides | `templates/guides/<name>.md.tmpl` |
| Example HCL snippets | `examples/resources/<name>/resource.tf` or `examples/data-sources/<name>/data-source.tf` |
| Schema descriptions | `MarkdownDescription` fields in `internal/provider/*.go` |

After editing any of the above, run:

```sh
make generate-docs                        # regenerate docs/
terraform fmt -check -recursive examples/ # ensure HCL is canonical
```

### Subcategories

Resources and data sources are grouped in the Registry sidebar via the `subcategory` frontmatter in their templates: `DNS`, `Domains`, `Contacts`, `Forwarding`, `Team`.

### Adding a new resource/data source

1. Create the Go implementation in `internal/provider/`
2. Register it in `provider.go` (`Resources()` / `DataSources()`)
3. Add an example in `examples/resources/<name>/resource.tf` (or `data-sources/<name>/data-source.tf`)
4. Optionally create a template in `templates/resources/<name>.md.tmpl` with a `subcategory` and import instructions
5. Run `make generate-docs` — if no template exists, tfplugindocs auto-generates one

## Excluded API surface area

Some OpusDNS API endpoints, SDK methods, and resource concepts are
**deliberately excluded** from this provider. They will not be implemented
as resources or data sources, regardless of SDK or API coverage.

The authoritative list lives at `EXCLUDED_API_FEATURES.md` in the repo
root. Treat it as load-bearing: every API-sync, audit, or feature-add
session must consult it **before** writing code.

### Required workflow before adding new coverage

1. **Read `EXCLUDED_API_FEATURES.md`** end-to-end. If the endpoint,
   resource name, or SDK reference appears there, **do not implement it**
   and cite the exclusion entry in the response to the user.
2. **Honour the rationale.** Even if the user asks for an excluded
   surface, surface the exclusion rationale and ask for explicit
   confirmation before proceeding. The exclusion can only be lifted by
   removing the entry in the same change that re-introduces the feature.
3. **Audit agents must respect this list.** When invoking the
   `opusdns-api-auditor` agent or any equivalent coverage audit, pass the
   contents (or path) of `EXCLUDED_API_FEATURES.md` as a constraint and
   verify the report flags excluded items as `excluded` rather than
   `missing`.

### Adding a new exclusion

Append an entry to `EXCLUDED_API_FEATURES.md` with the structure already
used (excluded surface area, API endpoints, SDK references, rationale,
decision date / commits). Keep the rationale concrete enough that a
future maintainer can decide whether the exclusion still applies.

### Currently excluded

- **Organization IP restrictions** — managed exclusively through the web
  interface. Covers `/v1/organizations/ip-restrictions[/{id}]` and the
  matching `Organizations.*IPRestriction*` SDK identifiers and
  `models.IPRestriction`. Decision history: commits `5caacc7`, `6080ad4`.

When this list grows, mirror the entry both here (short summary) and in
`EXCLUDED_API_FEATURES.md` (full rationale).
