package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure RegistrarCredentialsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &RegistrarCredentialsDataSource{}

// RegistrarCredentialsDataSource lists registrar credentials via
// `GET /v1/connect/registrars`, auto-paginated. The credential payloads
// themselves are never returned by the API and are not exposed here.
type RegistrarCredentialsDataSource struct {
	client *opusdns.Client
}

// RegistrarCredentialsDataSourceModel is the state shape for the list datasource.
type RegistrarCredentialsDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Registrar            types.String `tfsdk:"registrar"`
	RegistrarCredentials types.List   `tfsdk:"registrar_credentials"`
}

var registrarCredentialItemAttrTypes = map[string]attr.Type{
	"registrar_credential_id": types.StringType,
	"organization_id":         types.StringType,
	"name":                    types.StringType,
	"registrar":               types.StringType,
	"created_on":              types.StringType,
	"updated_on":              types.StringType,
}

// NewRegistrarCredentialsDataSource returns a new RegistrarCredentialsDataSource.
func NewRegistrarCredentialsDataSource() datasource.DataSource {
	return &RegistrarCredentialsDataSource{}
}

func (d *RegistrarCredentialsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registrar_credentials"
}

func (d *RegistrarCredentialsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists registrar credentials within the authenticated caller's organization (`GET /v1/connect/registrars`). " +
			"Results are auto-paginated. The credential payloads themselves are never returned by the API and are not exposed here.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"registrar": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter by registrar. One of `INTERNETX`, `MONIKER`, `DOMAIN_BESTELLSYSTEM`, `CENTRALNIC`, `OPUSDNS`, `ENOM`.",
				Validators: []validator.String{
					stringvalidator.OneOf("INTERNETX", "MONIKER", "DOMAIN_BESTELLSYSTEM", "CENTRALNIC", "OPUSDNS", "ENOM"),
				},
			},
			"registrar_credentials": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching registrar credentials.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"registrar_credential_id": schema.StringAttribute{Computed: true},
						"organization_id":         schema.StringAttribute{Computed: true},
						"name":                    schema.StringAttribute{Computed: true},
						"registrar":               schema.StringAttribute{Computed: true},
						"created_on":              schema.StringAttribute{Computed: true},
						"updated_on":              schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *RegistrarCredentialsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RegistrarCredentialsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RegistrarCredentialsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var registrar string
	if !data.Registrar.IsNull() && !data.Registrar.IsUnknown() {
		registrar = data.Registrar.ValueString()
	}

	creds, err := rawListRegistrarCredentials(ctx, d.client, registrar)
	if err != nil {
		resp.Diagnostics.AddError("Error listing registrar credentials", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: registrarCredentialItemAttrTypes}
	values := make([]attr.Value, len(creds))
	for i := range creds {
		c := &creds[i]
		obj, diags := types.ObjectValue(registrarCredentialItemAttrTypes, map[string]attr.Value{
			"registrar_credential_id": types.StringValue(c.RegistrarCredentialID),
			"organization_id":         types.StringValue(c.OrganizationID),
			"name":                    types.StringValue(c.Name),
			"registrar":               types.StringValue(c.Registrar),
			"created_on":              types.StringValue(c.CreatedOn.Format(time.RFC3339)),
			"updated_on":              types.StringValue(c.UpdatedOn.Format(time.RFC3339)),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		values[i] = obj
	}

	list, diags := types.ListValue(objType, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue("registrar_credentials")
	data.RegistrarCredentials = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
