package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure EmailForwardsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &EmailForwardsDataSource{}

// EmailForwardsDataSource lists all email forwards in a given zone via
// `ListEmailForwardsByZone`. There is no org-wide list mode; `zone_name` is
// required, matching the convention used by `opusdns_records`.
type EmailForwardsDataSource struct {
	client *opusdns.Client
}

// EmailForwardsDataSourceModel describes the data-source state.
type EmailForwardsDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	ZoneName      types.String `tfsdk:"zone_name"`
	EmailForwards types.List   `tfsdk:"email_forwards"`
}

// emailForwardListObjectAttrTypes is the attribute schema of each entry in
// the `email_forwards` list. Mirrors the singular data source's model,
// reusing `emailForwardAliasAttrTypes` for `aliases`.
var emailForwardListObjectAttrTypes = map[string]attr.Type{
	"id":               types.StringType,
	"email_forward_id": types.StringType,
	"hostname":         types.StringType,
	"enabled":          types.BoolType,
	"aliases":          types.ListType{ElemType: types.ObjectType{AttrTypes: emailForwardAliasAttrTypes}},
	"created_on":       types.StringType,
	"updated_on":       types.StringType,
}

var emailForwardListObjectType = types.ObjectType{AttrTypes: emailForwardListObjectAttrTypes}

// NewEmailForwardsDataSource returns a new EmailForwardsDataSource.
func NewEmailForwardsDataSource() datasource.DataSource {
	return &EmailForwardsDataSource{}
}

func (d *EmailForwardsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_email_forwards"
}

func (d *EmailForwardsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all email forwards configured for a zone via `GET /v1/zones/{zone}/email-forwards`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Synthetic identifier (the zone name).",
			},
			"zone_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the zone whose email forwards to list (e.g. `example.com`).",
			},
			"email_forwards": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All email forwards configured for the zone.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":               schema.StringAttribute{Computed: true, MarkdownDescription: "Same value as `email_forward_id`."},
						"email_forward_id": schema.StringAttribute{Computed: true, MarkdownDescription: "The unique identifier for the email forward."},
						"hostname":         schema.StringAttribute{Computed: true, MarkdownDescription: "The hostname this email forward is configured for."},
						"enabled":          schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether email forwarding is currently active."},
						"aliases": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "The list of email aliases configured for this hostname.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"alias_id":   schema.StringAttribute{Computed: true, MarkdownDescription: "The unique identifier for this alias."},
									"alias":      schema.StringAttribute{Computed: true, MarkdownDescription: "The alias part (e.g. `info` for `info@example.com`; `*` for a catch-all)."},
									"forward_to": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "The list of destination email addresses."},
								},
							},
						},
						"created_on": schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp of when the email forward was created."},
						"updated_on": schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp of when the email forward was last updated."},
					},
				},
			},
		},
	}
}

func (d *EmailForwardsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EmailForwardsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EmailForwardsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneName := data.ZoneName.ValueString()
	if zoneName == "" {
		resp.Diagnostics.AddError(
			"Invalid zone_name",
			"The `zone_name` attribute must be a non-empty zone name.",
		)
		return
	}

	efs, err := d.client.EmailForwards.ListEmailForwardsByZone(ctx, zoneName)
	if err != nil {
		resp.Diagnostics.AddError("Error listing email forwards", formatAPIError(err))
		return
	}

	values := make([]attr.Value, 0, len(efs))
	for i := range efs {
		obj, diags := emailForwardToObjectValue(ctx, &efs[i])
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		values = append(values, obj)
	}

	list, diags := types.ListValue(emailForwardListObjectType, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(zoneName)
	data.EmailForwards = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// emailForwardToObjectValue converts a single SDK EmailForward into the
// framework object value used by the `email_forwards` list attribute.
func emailForwardToObjectValue(ctx context.Context, ef *models.EmailForward) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	aliasObjType := types.ObjectType{AttrTypes: emailForwardAliasAttrTypes}
	aliasValues := make([]attr.Value, len(ef.Aliases))
	for i, a := range ef.Aliases {
		forwardToList, d := types.ListValueFrom(ctx, types.StringType, a.ForwardTo)
		diags.Append(d...)
		if diags.HasError() {
			return types.ObjectNull(emailForwardListObjectAttrTypes), diags
		}
		obj, d := types.ObjectValue(emailForwardAliasAttrTypes, map[string]attr.Value{
			"alias_id":   types.StringValue(string(a.EmailForwardAliasID)),
			"alias":      types.StringValue(a.Alias),
			"forward_to": forwardToList,
		})
		diags.Append(d...)
		if diags.HasError() {
			return types.ObjectNull(emailForwardListObjectAttrTypes), diags
		}
		aliasValues[i] = obj
	}
	aliasList, d := types.ListValue(aliasObjType, aliasValues)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(emailForwardListObjectAttrTypes), diags
	}

	obj, d := types.ObjectValue(emailForwardListObjectAttrTypes, map[string]attr.Value{
		"id":               types.StringValue(string(ef.EmailForwardID)),
		"email_forward_id": types.StringValue(string(ef.EmailForwardID)),
		"hostname":         types.StringValue(trimTrailingDot(ef.Hostname)),
		"enabled":          types.BoolValue(ef.Enabled),
		"aliases":          aliasList,
		"created_on":       types.StringValue(ef.CreatedOn.Format(rfc3339)),
		"updated_on":       types.StringValue(ef.UpdatedOn.Format(rfc3339)),
	})
	diags.Append(d...)
	return obj, diags
}
