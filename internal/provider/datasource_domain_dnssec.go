package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure DomainDNSSECDataSource satisfies the data source interface.
var _ datasource.DataSource = &DomainDNSSECDataSource{}

// DomainDNSSECDataSource exposes the current DNSSEC configuration for a
// domain (read-only). Returns whatever DomainsService.GetDNSSEC currently
// reports.
type DomainDNSSECDataSource struct {
	client *opusdns.Client
}

// DomainDNSSECDataSourceModel mirrors the resource model minus the
// intent-bearing `enabled` flag (which is config-only on the resource).
type DomainDNSSECDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	DomainRef types.String `tfsdk:"domain_ref"`
	Records   types.List   `tfsdk:"records"`
}

// NewDomainDNSSECDataSource returns a new DomainDNSSECDataSource.
func NewDomainDNSSECDataSource() datasource.DataSource {
	return &DomainDNSSECDataSource{}
}

func (d *DomainDNSSECDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_dnssec"
}

func (d *DomainDNSSECDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the current DNSSEC configuration for an OpusDNS domain.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier (the domain reference).",
			},
			"domain_ref": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Reference to the domain. Accepts either the domain id or the domain name.",
			},
			"records": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "DNSSEC records currently registered for the domain.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Server-assigned identifier for the DNSSEC record."},
						"record_type": schema.StringAttribute{Computed: true, MarkdownDescription: "Record type: `ds_data` or `key_data`."},
						"algorithm":   schema.Int64Attribute{Computed: true, MarkdownDescription: "DNSSEC algorithm number."},
						"digest":      schema.StringAttribute{Computed: true, MarkdownDescription: "DS record digest."},
						"digest_type": schema.Int64Attribute{Computed: true, MarkdownDescription: "DS digest type."},
						"flags":       schema.Int64Attribute{Computed: true, MarkdownDescription: "DNSKEY flags."},
						"key_tag":     schema.Int64Attribute{Computed: true, MarkdownDescription: "Key tag."},
						"protocol":    schema.Int64Attribute{Computed: true, MarkdownDescription: "DNSKEY protocol field."},
						"public_key":  schema.StringAttribute{Computed: true, MarkdownDescription: "DNSKEY public key."},
						"created_on":  schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 creation timestamp."},
						"updated_on":  schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 update timestamp."},
					},
				},
			},
		},
	}
}

func (d *DomainDNSSECDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *DomainDNSSECDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainDNSSECDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainRef := data.DomainRef.ValueString()
	if domainRef == "" {
		resp.Diagnostics.AddError("Missing domain reference", "`domain_ref` must be set.")
		return
	}

	apiRecords, err := d.client.Domains.GetDNSSEC(ctx, domainRef)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNSSEC data", formatAPIError(err))
		return
	}

	records, diags := dnssecRecordsToList(ctx, apiRecords)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(domainRef)
	data.Records = records

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
