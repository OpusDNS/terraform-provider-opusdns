# Copilot Instructions

## Build, test, and lint

```sh
make build          # go build ./...
make test           # go test ./... -v -count=1 -timeout 120s
make vet            # go vet ./...
make fmt            # gofmt -s -w .
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
- **Shared helpers** → `helpers.go`, `auth.go`

Every resource/data source is registered in `provider.go` inside `Resources()` / `DataSources()`.

The provider authenticates via three modes (priority order):
1. `client_secret` + `org_id` → direct `client_credentials` grant (preferred for automation)
2. `username` + `password` + `org_id` → full 3-step flow (mints a new API key each run)
3. `username` + `password` only → single-step password grant (user-token mode)

The SDK's `X-Api-Key` header is replaced with `Authorization: Bearer <token>` via a custom `bearerTransport` http.RoundTripper set in `Configure`.

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
