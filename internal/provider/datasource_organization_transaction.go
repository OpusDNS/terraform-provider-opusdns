package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure OrganizationTransactionDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &OrganizationTransactionDataSource{}

// OrganizationTransactionDataSource fetches a single billing transaction
// by id (`GET /v1/organizations/{organization_id}/transactions/{transaction_id}`).
type OrganizationTransactionDataSource struct {
	client *opusdns.Client
}

type OrganizationTransactionDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	OrganizationID       types.String `tfsdk:"organization_id"`
	Me                   types.Bool   `tfsdk:"me"`
	BillingTransactionID types.String `tfsdk:"billing_transaction_id"`

	ProductType      types.String `tfsdk:"product_type"`
	ProductReference types.String `tfsdk:"product_reference"`
	Action           types.String `tfsdk:"action"`
	Status           types.String `tfsdk:"status"`
	Price            types.String `tfsdk:"price"`
	TaxRate          types.String `tfsdk:"tax_rate"`
	TaxAmount        types.String `tfsdk:"tax_amount"`
	Amount           types.String `tfsdk:"amount"`
	Currency         types.String `tfsdk:"currency"`
	OriginalPrice    types.String `tfsdk:"original_price"`
	OriginalCurrency types.String `tfsdk:"original_currency"`
	ExchangeRate     types.String `tfsdk:"exchange_rate"`
	CreatedOn        types.String `tfsdk:"created_on"`
	UpdatedOn        types.String `tfsdk:"updated_on"`
	CompletedOn      types.String `tfsdk:"completed_on"`
}

func NewOrganizationTransactionDataSource() datasource.DataSource {
	return &OrganizationTransactionDataSource{}
}

func (d *OrganizationTransactionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_transaction"
}

func (d *OrganizationTransactionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single billing transaction by id " +
			"(`GET /v1/organizations/{organization_id}/transactions/{transaction_id}`). " +
			"Either set `organization_id` or `me = true`.",
		Attributes: map[string]schema.Attribute{
			"id":                     schema.StringAttribute{Computed: true, MarkdownDescription: "Mirror of `billing_transaction_id`."},
			"organization_id":        schema.StringAttribute{Optional: true, MarkdownDescription: "Organization id. Mutually exclusive with `me`."},
			"me":                     schema.BoolAttribute{Optional: true, MarkdownDescription: "When true, resolve the caller's organization id via `/v1/users/me`."},
			"billing_transaction_id": schema.StringAttribute{Required: true, MarkdownDescription: "Transaction id to fetch."},

			"product_type":      schema.StringAttribute{Computed: true, MarkdownDescription: "Product type."},
			"product_reference": schema.StringAttribute{Computed: true, MarkdownDescription: "Product reference (e.g. domain name), or empty if unset."},
			"action":            schema.StringAttribute{Computed: true, MarkdownDescription: "Action performed."},
			"status":            schema.StringAttribute{Computed: true, MarkdownDescription: "Transaction status."},
			"price":             schema.StringAttribute{Computed: true, MarkdownDescription: "Base price as a decimal string."},
			"tax_rate":          schema.StringAttribute{Computed: true, MarkdownDescription: "Tax rate applied, as a decimal string."},
			"tax_amount":        schema.StringAttribute{Computed: true, MarkdownDescription: "Tax amount, as a decimal string."},
			"amount":            schema.StringAttribute{Computed: true, MarkdownDescription: "Total amount including tax, as a decimal string."},
			"currency":          schema.StringAttribute{Computed: true, MarkdownDescription: "Currency code."},
			"original_price":    schema.StringAttribute{Computed: true, MarkdownDescription: "Original supplier-currency price, or empty if unset."},
			"original_currency": schema.StringAttribute{Computed: true, MarkdownDescription: "Currency the original price was in, or empty if unset."},
			"exchange_rate":     schema.StringAttribute{Computed: true, MarkdownDescription: "Exchange rate applied to convert original to billing currency, or empty if unset."},
			"created_on":        schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the transaction was created."},
			"updated_on":        schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the transaction was last updated."},
			"completed_on":      schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the transaction was completed, or empty if unset."},
		},
	}
}

func (d *OrganizationTransactionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := configureClientFromDataSourceProviderData(req.ProviderData, resp.Diagnostics.AddError); ok {
		d.client = c
	}
}

func (d *OrganizationTransactionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationTransactionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgID, _, diags := resolveOrgSelector(ctx, d.client, data.OrganizationID, data.Me)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if orgID == "" {
		resp.Diagnostics.AddError(
			"Missing organization selector",
			"`organization_id` or `me = true` is required for the transaction endpoint.",
		)
		return
	}

	txID := models.BillingTransactionID(data.BillingTransactionID.ValueString())
	tx, err := d.client.Organizations.GetTransaction(ctx, orgID, txID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization transaction", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(string(tx.BillingTransactionID))
	data.BillingTransactionID = types.StringValue(string(tx.BillingTransactionID))
	data.ProductType = types.StringValue(string(tx.ProductType))
	data.ProductReference = stringPtrToValue(tx.ProductReference)
	data.Action = types.StringValue(string(tx.Action))
	data.Status = types.StringValue(string(tx.Status))
	data.Price = types.StringValue(tx.Price)
	data.TaxRate = types.StringValue(tx.TaxRate)
	data.TaxAmount = types.StringValue(tx.TaxAmount)
	data.Amount = types.StringValue(tx.Amount)
	data.Currency = types.StringValue(string(tx.Currency))
	data.OriginalPrice = stringPtrToValue(tx.OriginalPrice)
	if tx.OriginalCurrency != nil {
		data.OriginalCurrency = types.StringValue(string(*tx.OriginalCurrency))
	} else {
		data.OriginalCurrency = types.StringNull()
	}
	data.ExchangeRate = stringPtrToValue(tx.ExchangeRate)
	data.CreatedOn = timePtrToValue(tx.CreatedOn)
	data.UpdatedOn = timePtrToValue(tx.UpdatedOn)
	data.CompletedOn = timePtrToValue(tx.CompletedOn)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
