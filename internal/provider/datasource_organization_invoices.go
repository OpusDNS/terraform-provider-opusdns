package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure OrganizationInvoicesDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &OrganizationInvoicesDataSource{}

// OrganizationInvoicesDataSource lists invoices for an organization
// (`GET /v1/organizations/{organization_id}/billing/invoices`).
//
// The SDK's `Organizations.ListInvoices` returns an `Invoice` struct whose
// field set is completely different from the live API response
// (`InvoiceResponse` in common/lib/utils/billing_gateway_client.py). This
// data source bypasses the SDK helper and decodes the actual
// `Pagination[InvoiceResponse]` payload via the raw HTTP client.
type OrganizationInvoicesDataSource struct {
	client *opusdns.Client
}

type OrganizationInvoicesDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Me             types.Bool   `tfsdk:"me"`
	Page           types.Int64  `tfsdk:"page"`
	PageSize       types.Int64  `tfsdk:"page_size"`
	Invoices       types.List   `tfsdk:"invoices"`
	TotalReturned  types.Int64  `tfsdk:"total_returned"`
}

var invoiceAttrTypes = map[string]attr.Type{
	"external_id":      types.StringType,
	"number":           types.StringType,
	"issuing_date":     types.StringType,
	"payment_due_date": types.StringType,
	"invoice_type":     types.StringType,
	"status":           types.StringType,
	"payment_status":   types.StringType,
	"payment_overdue":  types.BoolType,
	"currency":         types.StringType,
	"amount":           types.StringType,
	"fees_amount":      types.StringType,
	"taxes_amount":     types.StringType,
	"file_url":         types.StringType,
}

// invoiceWire mirrors the live API's InvoiceResponse
// (common/lib/utils/billing_gateway_client.py:659). Money values arrive as
// JSON numbers/strings; we render them as strings so callers don't lose
// precision via float64 round-tripping.
type invoiceWire struct {
	ExternalID     string      `json:"external_id"`
	Number         string      `json:"number"`
	IssuingDate    string      `json:"issuing_date"`
	PaymentDueDate string      `json:"payment_due_date"`
	InvoiceType    string      `json:"invoice_type"`
	Status         string      `json:"status"`
	PaymentStatus  string      `json:"payment_status"`
	PaymentOverdue bool        `json:"payment_overdue"`
	Currency       string      `json:"currency"`
	Amount         json.Number `json:"amount"`
	FeesAmount     json.Number `json:"fees_amount"`
	TaxesAmount    json.Number `json:"taxes_amount"`
	FileURL        *string     `json:"file_url,omitempty"`
}

// paginatedInvoicesWire matches Pagination[InvoiceResponse]'s `{results: [...]}`.
type paginatedInvoicesWire struct {
	Results []invoiceWire `json:"results"`
}

func NewOrganizationInvoicesDataSource() datasource.DataSource {
	return &OrganizationInvoicesDataSource{}
}

func (d *OrganizationInvoicesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_invoices"
}

func (d *OrganizationInvoicesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists billing invoices for an organization " +
			"(`GET /v1/organizations/{organization_id}/billing/invoices`). Either set " +
			"`organization_id` or `me = true`. Returns a single page; use `page`/`page_size` " +
			"to walk results.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"organization_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Organization id. Mutually exclusive with `me`."},
			"me":              schema.BoolAttribute{Optional: true, MarkdownDescription: "When true, resolve the caller's organization id via `/v1/users/me`."},
			"page":            schema.Int64Attribute{Optional: true, MarkdownDescription: "1-indexed page number."},
			"page_size":       schema.Int64Attribute{Optional: true, MarkdownDescription: "Number of entries per page."},
			"total_returned": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of invoices returned (length of `invoices`).",
			},
			"invoices": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Invoices on this page.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"external_id":      schema.StringAttribute{Computed: true, MarkdownDescription: "External (Lago) invoice identifier."},
						"number":           schema.StringAttribute{Computed: true, MarkdownDescription: "Invoice number."},
						"issuing_date":     schema.StringAttribute{Computed: true, MarkdownDescription: "ISO-8601 date when the invoice was issued."},
						"payment_due_date": schema.StringAttribute{Computed: true, MarkdownDescription: "ISO-8601 date the invoice payment is due."},
						"invoice_type":     schema.StringAttribute{Computed: true, MarkdownDescription: "Invoice type (`advance_charges`, `progressive_billing`, ...)."},
						"status":           schema.StringAttribute{Computed: true, MarkdownDescription: "Invoice status (`draft`, `finalized`, ...)."},
						"payment_status":   schema.StringAttribute{Computed: true, MarkdownDescription: "Payment status (`succeeded`, `failed`, ...)."},
						"payment_overdue":  schema.BoolAttribute{Computed: true, MarkdownDescription: "True when payment is overdue."},
						"currency":         schema.StringAttribute{Computed: true, MarkdownDescription: "Currency code (e.g. `EUR`, `USD`)."},
						"amount":           schema.StringAttribute{Computed: true, MarkdownDescription: "Total invoice amount as a decimal string."},
						"fees_amount":      schema.StringAttribute{Computed: true, MarkdownDescription: "Fees component as a decimal string."},
						"taxes_amount":     schema.StringAttribute{Computed: true, MarkdownDescription: "Taxes component as a decimal string."},
						"file_url":         schema.StringAttribute{Computed: true, MarkdownDescription: "URL to the invoice PDF, or empty if not available."},
					},
				},
			},
		},
	}
}

func (d *OrganizationInvoicesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := configureClientFromDataSourceProviderData(req.ProviderData, resp.Diagnostics.AddError); ok {
		d.client = c
	}
}

func (d *OrganizationInvoicesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationInvoicesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgID, idLabel, diags := resolveOrgSelector(ctx, d.client, data.OrganizationID, data.Me)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if orgID == "" {
		resp.Diagnostics.AddError(
			"Missing organization selector",
			"`organization_id` or `me = true` is required for the invoices endpoint.",
		)
		return
	}

	path := d.client.HTTPClient().BuildPath("organizations", string(orgID), "billing", "invoices")
	query := url.Values{}
	if !data.Page.IsNull() && !data.Page.IsUnknown() {
		query.Set("page", strconv.Itoa(int(data.Page.ValueInt64())))
	}
	if !data.PageSize.IsNull() && !data.PageSize.IsUnknown() {
		query.Set("page_size", strconv.Itoa(int(data.PageSize.ValueInt64())))
	}

	httpResp, err := d.client.HTTPClient().Get(ctx, path, query)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching organization invoices", formatAPIError(err))
		return
	}
	var paginated paginatedInvoicesWire
	if err := d.client.HTTPClient().DecodeResponse(httpResp, &paginated); err != nil {
		var raw json.RawMessage
		_ = d.client.HTTPClient().DecodeResponse(httpResp, &raw)
		resp.Diagnostics.AddError(
			"Error decoding organization invoices response",
			fmt.Sprintf("expected a Pagination[InvoiceResponse] envelope: %s\nraw body: %s",
				err.Error(), string(raw)),
		)
		return
	}

	objType := types.ObjectType{AttrTypes: invoiceAttrTypes}
	values := make([]attr.Value, len(paginated.Results))
	for i, e := range paginated.Results {
		obj, ed := invoiceWireToObject(e)
		resp.Diagnostics.Append(ed...)
		if resp.Diagnostics.HasError() {
			return
		}
		values[i] = obj
	}
	list, ld := types.ListValue(objType, values)
	resp.Diagnostics.Append(ld...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue("invoices:" + idLabel)
	data.Invoices = list
	data.TotalReturned = types.Int64Value(int64(len(paginated.Results)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func invoiceWireToObject(e invoiceWire) (types.Object, diag.Diagnostics) {
	fileURL := ""
	if e.FileURL != nil {
		fileURL = *e.FileURL
	}
	return types.ObjectValue(invoiceAttrTypes, map[string]attr.Value{
		"external_id":      types.StringValue(e.ExternalID),
		"number":           types.StringValue(e.Number),
		"issuing_date":     types.StringValue(e.IssuingDate),
		"payment_due_date": types.StringValue(e.PaymentDueDate),
		"invoice_type":     types.StringValue(e.InvoiceType),
		"status":           types.StringValue(e.Status),
		"payment_status":   types.StringValue(e.PaymentStatus),
		"payment_overdue":  types.BoolValue(e.PaymentOverdue),
		"currency":         types.StringValue(e.Currency),
		"amount":           types.StringValue(string(e.Amount)),
		"fees_amount":      types.StringValue(string(e.FeesAmount)),
		"taxes_amount":     types.StringValue(string(e.TaxesAmount)),
		"file_url":         types.StringValue(fileURL),
	})
}
