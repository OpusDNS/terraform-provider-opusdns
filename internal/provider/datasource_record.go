package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure DNSRecordDataSource satisfies the data source interface.
var _ datasource.DataSource = &DNSRecordDataSource{}

// DNSRecordDataSource looks up a single RRSet (the records sharing one
// name+type) within an OpusDNS-hosted zone.
//
// The API does not expose a per-RRSet endpoint, so the data source fetches
// the full zone via DNS.GetZone and filters client-side. For zones with many
// RRSets prefer this over `data.opusdns_records` only when you need exactly
// one set, since both incur the same single API call.
type DNSRecordDataSource struct {
	client *opusdns.Client
}

// DNSRecordDataSourceModel mirrors the resource model.
type DNSRecordDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	ZoneName        types.String `tfsdk:"zone_name"`
	Name            types.String `tfsdk:"name"`
	Type            types.String `tfsdk:"type"`
	TTL             types.Int64  `tfsdk:"ttl"`
	Records         types.List   `tfsdk:"records"`
	Protected       types.Bool   `tfsdk:"protected"`
	ProtectedReason types.String `tfsdk:"protected_reason"`
}

// NewDNSRecordDataSource returns a new DNSRecordDataSource.
func NewDNSRecordDataSource() datasource.DataSource {
	return &DNSRecordDataSource{}
}

func (d *DNSRecordDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (d *DNSRecordDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single DNS record set (RRSet) by zone, name, and type. Returns an error if no matching RRSet exists.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite identifier in the format `zone_name/name/type`.",
			},
			"zone_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the zone containing the record (e.g. `example.com`).",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Record name relative to the zone (e.g. `www` or `@` for apex).",
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "DNS record type (e.g. `A`, `AAAA`, `CNAME`, `MX`, `TXT`).",
			},
			"ttl": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Time-to-live in seconds.",
			},
			"records": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Record data values for this RRSet (e.g. IP addresses for `A`, hostnames for `CNAME`).",
			},
			"protected": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the RRSet is protected from modification.",
			},
			"protected_reason": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Reason the RRSet is protected, when applicable.",
			},
		},
	}
}

func (d *DNSRecordDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DNSRecordDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DNSRecordDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneName := data.ZoneName.ValueString()
	recName := data.Name.ValueString()
	rtype := data.Type.ValueString()

	zone, err := d.client.DNS.GetZone(ctx, zoneName)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS zone", formatAPIError(err))
		return
	}

	rrset := findRRSet(zone.RRSets, recName, models.RRSetType(rtype), zoneName)
	if rrset == nil {
		resp.Diagnostics.AddError(
			"DNS record not found",
			fmt.Sprintf("No %s record named %q exists in zone %q.", rtype, recName, zoneName),
		)
		return
	}

	rdatas := make([]attr.Value, len(rrset.Records))
	for i, r := range rrset.Records {
		rdatas[i] = types.StringValue(normalizeRData(string(rrset.Type), r.RData))
	}
	recordList, diags := types.ListValue(types.StringType, rdatas)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(recordID(zoneName, recName, rtype))
	data.TTL = types.Int64Value(int64(rrset.TTL))
	data.Records = recordList
	data.Protected = types.BoolValue(rrset.Protected)
	data.ProtectedReason = stringPtrToValue(rrset.ProtectedReason)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
