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

// Ensure EmailForwardDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &EmailForwardDataSource{}

// EmailForwardDataSource fetches a single email forward by its opaque
// email_forward_id via `GET /v1/email-forwards/{id}`.
type EmailForwardDataSource struct {
	client *opusdns.Client
}

// EmailForwardDataSourceModel is the data-source state shape. It mirrors
// the resource model but adds the timestamps that the resource does not
// surface (`created_on`, `updated_on`).
type EmailForwardDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	EmailForwardID types.String `tfsdk:"email_forward_id"`
	Hostname       types.String `tfsdk:"hostname"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Aliases        types.List   `tfsdk:"aliases"`
	CreatedOn      types.String `tfsdk:"created_on"`
	UpdatedOn      types.String `tfsdk:"updated_on"`
}

// NewEmailForwardDataSource returns a new EmailForwardDataSource.
func NewEmailForwardDataSource() datasource.DataSource {
	return &EmailForwardDataSource{}
}

func (d *EmailForwardDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_email_forward"
}

func (d *EmailForwardDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single email forward by its opaque `email_forward_id`. " +
			"Use `opusdns_email_forwards` to list all email forwards in a zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same value as `email_forward_id`.",
			},
			"email_forward_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier for the email forward (e.g. `email_forward_...`).",
			},
			"hostname": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The hostname this email forward is configured for.",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether email forwarding is currently active.",
			},
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
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp of when the email forward was created.",
			},
			"updated_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp of when the email forward was last updated.",
			},
		},
	}
}

func (d *EmailForwardDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EmailForwardDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EmailForwardDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.EmailForwardID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError(
			"Invalid email_forward_id",
			"The `email_forward_id` attribute must be a non-empty identifier.",
		)
		return
	}

	ef, err := d.client.EmailForwards.GetEmailForward(ctx, models.EmailForwardID(id))
	if err != nil {
		resp.Diagnostics.AddError("Error reading email forward", formatAPIError(err))
		return
	}

	resp.Diagnostics.Append(setEmailForwardDataSourceState(ctx, &data, ef)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

// setEmailForwardDataSourceState populates the data-source model from an
// API response, including the timestamps that the resource state builder
// omits. Reuses `emailForwardAliasAttrTypes` from the resource so the
// nested `aliases` shape is identical.
func setEmailForwardDataSourceState(ctx context.Context, data *EmailForwardDataSourceModel, ef *models.EmailForward) diag.Diagnostics {
	var diags diag.Diagnostics

	data.ID = types.StringValue(string(ef.EmailForwardID))
	data.EmailForwardID = types.StringValue(string(ef.EmailForwardID))
	data.Hostname = types.StringValue(ef.Hostname)
	data.Enabled = types.BoolValue(ef.Enabled)
	data.CreatedOn = types.StringValue(ef.CreatedOn.Format(rfc3339))
	data.UpdatedOn = types.StringValue(ef.UpdatedOn.Format(rfc3339))

	aliasObjType := types.ObjectType{AttrTypes: emailForwardAliasAttrTypes}
	aliasValues := make([]attr.Value, len(ef.Aliases))
	for i, a := range ef.Aliases {
		forwardToList, d := types.ListValueFrom(ctx, types.StringType, a.ForwardTo)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		obj, d := types.ObjectValue(emailForwardAliasAttrTypes, map[string]attr.Value{
			"alias_id":   types.StringValue(string(a.EmailForwardAliasID)),
			"alias":      types.StringValue(a.Alias),
			"forward_to": forwardToList,
		})
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		aliasValues[i] = obj
	}

	aliasList, d := types.ListValue(aliasObjType, aliasValues)
	diags.Append(d...)
	data.Aliases = aliasList
	return diags
}

// rfc3339 is the RFC3339 timestamp layout used consistently across data
// sources that surface API timestamps as strings.
const rfc3339 = "2006-01-02T15:04:05Z07:00"
