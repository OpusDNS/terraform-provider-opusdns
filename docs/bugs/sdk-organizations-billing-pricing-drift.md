# Bug: opusdns-go-client v1.0.10 Organizations service drifts from API for attributes, invoices, pricing

## Summary

Three of the `Organizations` service helpers in `opusdns-go-client@v1.0.10` do not match the live OpusDNS API. They build wrong paths, expect wrong response shapes, and/or omit query parameters. As a result they cannot be used by downstream Go callers (including the Terraform provider) to read organization attributes, invoices, or product pricing. The provider works around all three by hitting the endpoints via the raw HTTP client.

## Severity

Medium for SDK consumers — all three calls return errors or empty data against the real backend. The Terraform provider implements working data sources via `client.HTTPClient().BuildPath/Get/DecodeResponse`, so users are unblocked, but every new consumer hits the same problem and has to re-discover and re-implement the workaround.

This issue tracks three independent drifts under one umbrella; they share a fix discipline (regenerate from the OpenAPI spec) but can be patched in isolation.

## Environment

- SDK: `github.com/opusdns/opusdns-go-client v1.0.10`
- API: live `https://api.opusdns.local` (matches `~/code/opusdns/api` working copy at the time of this report)
- terraform-provider-opusdns: branch `remove-organization-ip-restriction-resource`

## Drift 1 — `GetAttributes` / `UpdateAttributes`

### What the SDK does

- `OrganizationsService.GetAttributes(ctx, orgID)` builds path `organizations/attributes/{orgID}` and decodes into a wrapper struct `OrganizationAttributesResponse{ Attributes []OrganizationAttribute }`.
- `OrganizationsService.UpdateAttributes(ctx, orgID, req)` builds the same path with PATCH.

References: `opusdns/service_organizations.go:244-275`, `models/organizations.go:322-326`.

### What the API actually exposes

Two endpoint shapes, both returning a **bare list** of `OrganizationAttributeResponse`:

| Endpoint | Method | Returns |
|---|---|---|
| `/v1/organizations/attributes` | GET / PATCH | `list[OrganizationAttributeResponse]` for the caller's organization |
| `/v1/organizations/{organization_id}/attributes` | GET / PATCH | `list[OrganizationAttributeResponse]` for the specified organization |

References: `api/api/organization/v1_routes.py:143-275`, `common/models/account/organization_attribute.py:46-51`.

### Impact

The path the SDK builds (`/v1/organizations/attributes/{orgID}`) does not exist; the request 404s. Even if it did, the response wouldn't decode because the API returns `[…]` (bare array), not `{"attributes": […]}`.

The SDK also doesn't expose the optional `keys` query parameter the API supports for narrowing the response.

### Workaround in provider

`internal/provider/datasource_organization_attributes.go` uses `client.HTTPClient().BuildPath("organizations"[, orgID], "attributes")` with optional `keys` query params and decodes into a private `[]organizationAttributeWire` slice mirroring the actual API shape (including `value` as `json.RawMessage` — see drift discussion below).

## Drift 2 — `ListInvoices`

### What the SDK does

`OrganizationsService.ListInvoices(ctx, orgID)` builds `organizations/{orgID}/billing/invoices` (path correct) and decodes into `InvoiceListResponse{ Results []Invoice }` with `Invoice` fields:

```go
type Invoice struct {
    InvoiceID     TypeID    `json:"invoice_id"`
    InvoiceNumber string    `json:"invoice_number"`
    Status        string    `json:"status"`
    Amount        string    `json:"amount"`
    Currency      Currency  `json:"currency"`
    DueDate       *time.Time `json:"due_date,omitempty"`
    PaidOn        *time.Time `json:"paid_on,omitempty"`
    CreatedOn     *time.Time `json:"created_on,omitempty"`
}
```

References: `opusdns/service_organizations.go:343-358`, `models/organizations.go:495-519`.

### What the API actually returns

`Pagination[InvoiceResponse]` where each entry uses an entirely different field set:

```python
class InvoiceResponse(BillingGatewayResponse):
    external_id: str             # alias lago_id
    number: str
    issuing_date: datetime
    payment_due_date: datetime
    invoice_type: InvoiceResponseType
    status: InvoiceResponseStatus
    payment_status: InvoiceResponsePaymentStatus
    payment_overdue: bool
    currency: Currency
    amount: Decimal
    fees_amount: Decimal
    taxes_amount: Decimal
    file_url: str | None
```

References: `api/api/organization/billing_routes.py:129-149`, `common/lib/utils/billing_gateway_client.py:659-674`.

### Impact

The SDK decode succeeds against malformed inputs (Go silently ignores unknown fields) but produces an `Invoice` slice with every field zero-valued. Callers get a list of empty objects and have no way to read `payment_status`, `payment_due_date`, `file_url`, etc. The SDK also doesn't surface `page`/`page_size` query parameters.

### Workaround in provider

`internal/provider/datasource_organization_invoices.go` decodes the actual `Pagination[InvoiceResponse]` envelope via private `invoiceWire` and `paginatedInvoicesWire` structs and accepts `page`/`page_size` filters. Money fields are rendered as decimal strings via `json.Number` to preserve precision.

## Drift 3 — `GetPricing`

### What the SDK does

`OrganizationsService.GetPricing(ctx, orgID, productType)` builds `organizations/{orgID}/pricing/product-type/{productType}` (path correct) and decodes into:

```go
type ProductPricing struct {
    ProductType      string               `json:"product_type"`
    ProductReference *string              `json:"product_reference,omitempty"`
    Actions          map[string]PriceInfo `json:"actions,omitempty"`
}

type PriceInfo struct {
    Price      string   `json:"price"`
    Currency   Currency `json:"currency"`
    TaxRate    *string  `json:"tax_rate,omitempty"`
    TotalPrice *string  `json:"total_price,omitempty"`
}
```

References: `opusdns/service_organizations.go:360-374`, `models/organizations.go:328-353`.

### What the API actually returns

`GetPricesResponse{ prices: [PriceInfo{...}] }` where each `PriceInfo` is a flat record per (product_type, product_action, product_class) tuple with optional `period`:

```python
class PriceInfo(BaseModel):
    product_type: str
    product_action: str | None = None
    product_class: str | None = None
    price: Decimal
    currency: str
    period: PricingPeriod | None  # {value:int, unit:str}

class GetPricesResponse(BillingGatewayResponse):
    prices: list[PriceInfo]
```

References: `api/api/organization/pricing_routes.py:45-89`, `common/lib/utils/billing_gateway_client.py:286-314`.

### Impact

Two problems:

1. Shape mismatch — caller receives an empty `ProductPricing{Actions: nil}`.
2. The SDK doesn't accept the `product_action` and `product_class` query parameters the API supports. Without them, requests for `product_type=domain` return prices for every TLD × action pair, which is unmanageable.

### Workaround in provider

`internal/provider/datasource_organization_pricing.go` decodes the live `GetPricesResponse{prices: [...]}` envelope via private `pricingResponseWire` and forwards optional `product_action`/`product_class` query parameters. Prices are surfaced as decimal strings via `json.Number`.

## Related (non-blocker) — attribute `value` typing

`OrganizationAttribute` in the SDK types `Value` as `string`, but the API stores it as arbitrary JSON (`JsonValue | None` per `common/models/account/organization_attribute.py:30`). Provider works around this by treating `value` as opaque JSON (`json.RawMessage` on the wire, surfaced as `value_json` for `jsondecode()`).

## Suggested acceptance criteria

1. `Organizations.GetAttributes` and `UpdateAttributes` are regenerated against the real OpenAPI spec: they build `organizations/attributes` and `organizations/{organization_id}/attributes`, accept an optional `keys` query param, and return / accept a bare `[]OrganizationAttribute` slice. `Value` should be typed as `json.RawMessage` (or `interface{}`) to handle JSON payloads.
2. `Invoice` model is regenerated to match `InvoiceResponse` (`external_id`, `number`, `issuing_date`, `payment_due_date`, `invoice_type`, `status`, `payment_status`, `payment_overdue`, `currency`, `amount`, `fees_amount`, `taxes_amount`, `file_url`). `ListInvoices` accepts `page`/`page_size`.
3. `GetPricing` is regenerated to return `GetPricesResponse{Prices: []PriceInfo}` (matching the API), and accepts `product_action`/`product_class` query params; per-item `Period` is exposed as `{Value, Unit}`.
4. Once any of the above lands, the corresponding provider data source can drop the raw-HTTP workaround and call the SDK helper. Track this in `EXCLUDED_API_FEATURES.md` parity if needed.

## Provider workaround locations (for cleanup after SDK fix)

- `internal/provider/datasource_organization_attributes.go` — drop raw HTTP, drop `organizationAttributeWire`, switch to SDK call once attributes are fixed.
- `internal/provider/datasource_organization_invoices.go` — drop raw HTTP, drop `invoiceWire`/`paginatedInvoicesWire`, switch to SDK call once `Invoice` is fixed.
- `internal/provider/datasource_organization_pricing.go` — drop raw HTTP, drop `pricingResponseWire`/`pricingPriceWire`, switch to SDK call once `GetPricing` is fixed.
- `internal/provider/datasource_tld_portfolio.go` — same pattern; pre-existing drift (`TLDs.GetPortfolio` wrapper vs bare array).
