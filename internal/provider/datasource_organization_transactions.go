package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure OrganizationTransactionsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &OrganizationTransactionsDataSource{}

// OrganizationTransactionsDataSource lists billing transactions for an
// organization (`GET /v1/organizations/{organization_id}/transactions`).
// Supports server-side filters for product type, action, status, and a
// creation-time window, plus pagination.
type OrganizationTransactionsDataSource struct {
	client *opusdns.Client
}

type OrganizationTransactionsDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Me             types.Bool   `tfsdk:"me"`
	Page           types.Int64  `tfsdk:"page"`
	PageSize       types.Int64  `tfsdk:"page_size"`
	SortBy         types.String `tfsdk:"sort_by"`
	SortOrder      types.String `tfsdk:"sort_order"`
	ProductType    types.String `tfsdk:"product_type"`
	Action         types.String `tfsdk:"action"`
	Status         types.String `tfsdk:"status"`
	CreatedAfter   types.String `tfsdk:"created_after"`
	CreatedBefore  types.String `tfsdk:"created_before"`
	Transactions   types.List   `tfsdk:"transactions"`
	TotalReturned  types.Int64  `tfsdk:"total_returned"`
}

var billingTransactionAttrTypes = map[string]attr.Type{
	"billing_transaction_id": types.StringType,
	"product_type":           types.StringType,
	"product_reference":      types.StringType,
	"action":                 types.StringType,
	"status":                 types.StringType,
	"price":                  types.StringType,
	"tax_rate":               types.StringType,
	"tax_amount":             types.StringType,
	"amount":                 types.StringType,
	"currency":               types.StringType,
	"original_price":         types.StringType,
	"original_currency":      types.StringType,
	"exchange_rate":          types.StringType,
	"created_on":             types.StringType,
	"updated_on":             types.StringType,
	"completed_on":           types.StringType,
}

func NewOrganizationTransactionsDataSource() datasource.DataSource {
	return &OrganizationTransactionsDataSource{}
}

func (d *OrganizationTransactionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_transactions"
}

func (d *OrganizationTransactionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists billing transactions for an organization " +
			"(`GET /v1/organizations/{organization_id}/transactions`). Either set " +
			"`organization_id` or `me = true`. Returns a single page; use `page`/`page_size` " +
			"and the filter attributes to narrow results.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"organization_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Organization id to look up. Mutually exclusive with `me`."},
			"me":              schema.BoolAttribute{Optional: true, MarkdownDescription: "When true, resolve the caller's organization id via `/v1/users/me`."},

			"page":       schema.Int64Attribute{Optional: true, MarkdownDescription: "1-indexed page number."},
			"page_size":  schema.Int64Attribute{Optional: true, MarkdownDescription: "Number of entries per page."},
			"sort_by":    schema.StringAttribute{Optional: true, MarkdownDescription: "Field to sort by: `product_type`, `action`, `status`, `created_on`, `completed_on`."},
			"sort_order": schema.StringAttribute{Optional: true, MarkdownDescription: "Sort direction: `asc` or `desc`."},

			"product_type": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by product type (e.g. `domain`, `email_forward`, `domain_forward`, `zones`, `account_wallet`)."},
			"action":       schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by action (e.g. `create`, `renew`, `transfer`, `restore`, `wallet_top_up`)."},
			"status":       schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by status (`pending`, `succeeded`, `failed`, `canceled`)."},

			"created_after":  schema.StringAttribute{Optional: true, MarkdownDescription: "RFC3339 timestamp; only transactions created strictly after this are returned."},
			"created_before": schema.StringAttribute{Optional: true, MarkdownDescription: "RFC3339 timestamp; only transactions created strictly before this are returned."},

			"total_returned": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of transactions returned (length of `transactions`).",
			},
			"transactions": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Transactions on this page.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"billing_transaction_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Unique transaction id."},
						"product_type":           schema.StringAttribute{Computed: true, MarkdownDescription: "Product type."},
						"product_reference":      schema.StringAttribute{Computed: true, MarkdownDescription: "Product reference (e.g. domain name), or empty if unset."},
						"action":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Action performed."},
						"status":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Transaction status."},
						"price":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Base price as a decimal string."},
						"tax_rate":               schema.StringAttribute{Computed: true, MarkdownDescription: "Tax rate applied, as a decimal string."},
						"tax_amount":             schema.StringAttribute{Computed: true, MarkdownDescription: "Tax amount, as a decimal string."},
						"amount":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Total amount including tax, as a decimal string."},
						"currency":               schema.StringAttribute{Computed: true, MarkdownDescription: "Currency code (e.g. `EUR`, `USD`)."},
						"original_price":         schema.StringAttribute{Computed: true, MarkdownDescription: "Original supplier-currency price, or empty if unset."},
						"original_currency":      schema.StringAttribute{Computed: true, MarkdownDescription: "Currency the original price was in, or empty if unset."},
						"exchange_rate":          schema.StringAttribute{Computed: true, MarkdownDescription: "Exchange rate applied to convert original to billing currency, or empty if unset."},
						"created_on":             schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the transaction was created."},
						"updated_on":             schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the transaction was last updated."},
						"completed_on":           schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the transaction was completed, or empty if unset."},
					},
				},
			},
		},
	}
}

func (d *OrganizationTransactionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := configureClientFromDataSourceProviderData(req.ProviderData, resp.Diagnostics.AddError); ok {
		d.client = c
	}
}

func (d *OrganizationTransactionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationTransactionsDataSourceModel
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
			"`organization_id` or `me = true` is required for the transactions endpoint.",
		)
		return
	}

	opts := &models.ListTransactionsOptions{}
	if !data.Page.IsNull() && !data.Page.IsUnknown() {
		opts.Page = int(data.Page.ValueInt64())
	}
	if !data.PageSize.IsNull() && !data.PageSize.IsUnknown() {
		opts.PageSize = int(data.PageSize.ValueInt64())
	}
	if v := data.SortBy.ValueString(); v != "" {
		opts.SortBy = models.BillingTransactionSortField(v)
	}
	if v := data.SortOrder.ValueString(); v != "" {
		opts.SortOrder = models.SortOrder(v)
	}
	if v := data.ProductType.ValueString(); v != "" {
		opts.ProductType = models.BillingTransactionProductType(v)
	}
	if v := data.Action.ValueString(); v != "" {
		opts.Action = models.BillingTransactionAction(v)
	}
	if v := data.Status.ValueString(); v != "" {
		opts.Status = models.BillingTransactionStatus(v)
	}
	var pd diag.Diagnostics
	if opts.CreatedAfter, pd = parseOptionalRFC3339(data.CreatedAfter, "created_after"); pd.HasError() {
		resp.Diagnostics.Append(pd...)
		return
	}
	if opts.CreatedBefore, pd = parseOptionalRFC3339(data.CreatedBefore, "created_before"); pd.HasError() {
		resp.Diagnostics.Append(pd...)
		return
	}

	result, err := d.client.Organizations.ListTransactions(ctx, orgID, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing organization transactions", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: billingTransactionAttrTypes}
	values := make([]attr.Value, len(result.Results))
	for i, t := range result.Results {
		obj, td := billingTransactionToObject(t)
		resp.Diagnostics.Append(td...)
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

	data.ID = types.StringValue("transactions:" + idLabel)
	data.Transactions = list
	data.TotalReturned = types.Int64Value(int64(len(result.Results)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func billingTransactionToObject(t models.BillingTransaction) (types.Object, diag.Diagnostics) {
	productRef := ""
	if t.ProductReference != nil {
		productRef = *t.ProductReference
	}
	originalPrice := ""
	if t.OriginalPrice != nil {
		originalPrice = *t.OriginalPrice
	}
	originalCurrency := ""
	if t.OriginalCurrency != nil {
		originalCurrency = string(*t.OriginalCurrency)
	}
	exchangeRate := ""
	if t.ExchangeRate != nil {
		exchangeRate = *t.ExchangeRate
	}
	createdOn := ""
	if t.CreatedOn != nil {
		createdOn = t.CreatedOn.Format("2006-01-02T15:04:05Z07:00")
	}
	updatedOn := ""
	if t.UpdatedOn != nil {
		updatedOn = t.UpdatedOn.Format("2006-01-02T15:04:05Z07:00")
	}
	completedOn := ""
	if t.CompletedOn != nil {
		completedOn = t.CompletedOn.Format("2006-01-02T15:04:05Z07:00")
	}
	return types.ObjectValue(billingTransactionAttrTypes, map[string]attr.Value{
		"billing_transaction_id": types.StringValue(string(t.BillingTransactionID)),
		"product_type":           types.StringValue(string(t.ProductType)),
		"product_reference":      types.StringValue(productRef),
		"action":                 types.StringValue(string(t.Action)),
		"status":                 types.StringValue(string(t.Status)),
		"price":                  types.StringValue(t.Price),
		"tax_rate":               types.StringValue(t.TaxRate),
		"tax_amount":             types.StringValue(t.TaxAmount),
		"amount":                 types.StringValue(t.Amount),
		"currency":               types.StringValue(string(t.Currency)),
		"original_price":         types.StringValue(originalPrice),
		"original_currency":      types.StringValue(originalCurrency),
		"exchange_rate":          types.StringValue(exchangeRate),
		"created_on":             types.StringValue(createdOn),
		"updated_on":             types.StringValue(updatedOn),
		"completed_on":           types.StringValue(completedOn),
	})
}
