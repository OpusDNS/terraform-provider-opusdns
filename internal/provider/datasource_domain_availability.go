package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure DomainAvailabilityDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &DomainAvailabilityDataSource{}

// DomainAvailabilityDataSource defines the data source implementation.
type DomainAvailabilityDataSource struct {
	client *opusdns.Client
}

// DomainAvailabilityDataSourceModel describes the data source data model.
type DomainAvailabilityDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Domain      types.String `tfsdk:"domain"`
	Status      types.String `tfsdk:"status"`
	IsAvailable types.Bool   `tfsdk:"is_available"`
}

// NewDomainAvailabilityDataSource returns a new DomainAvailabilityDataSource.
func NewDomainAvailabilityDataSource() datasource.DataSource {
	return &DomainAvailabilityDataSource{}
}

func (d *DomainAvailabilityDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_availability"
}

func (d *DomainAvailabilityDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Checks whether a single domain is available for registration via the OpusDNS `/v1/availability` endpoint. " +
			"Useful as a precondition for an `opusdns_domain` resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The domain name (used as the unique identifier).",
			},
			"domain": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The fully-qualified domain name to check (e.g. `example.com`).",
			},
			"status": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "The raw availability status returned by the API. One of " +
					"`available`, `unavailable`, `market_available`, `tmch_claim`, or `error`.",
			},
			"is_available": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Convenience boolean: `true` only when `status` is `available`.",
			},
		},
	}
}

func (d *DomainAvailabilityDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *DomainAvailabilityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainAvailabilityDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain := data.Domain.ValueString()
	if domain == "" {
		resp.Diagnostics.AddError(
			"Invalid domain",
			"The `domain` attribute must be a non-empty fully-qualified domain name.",
		)
		return
	}

	result, err := d.client.Availability.CheckSingleAvailability(ctx, domain)
	if err != nil {
		resp.Diagnostics.AddError("Error checking domain availability", formatAPIError(err))
		return
	}

	// Defensive: the SDK only returns an error when the request fails; an
	// empty result struct still surfaces as nil-error with empty fields.
	if result == nil || result.Domain == "" {
		resp.Diagnostics.AddError(
			"Empty availability response",
			fmt.Sprintf("The /v1/availability endpoint returned no result for %q.", domain),
		)
		return
	}

	data.ID = types.StringValue(result.Domain)
	data.Domain = types.StringValue(result.Domain)
	data.Status = types.StringValue(string(result.Status))
	data.IsAvailable = types.BoolValue(result.Status == models.AvailabilityStatusAvailable)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
