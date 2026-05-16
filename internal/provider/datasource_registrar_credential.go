package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure RegistrarCredentialDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &RegistrarCredentialDataSource{}

// RegistrarCredentialDataSource reads a single registrar credential by id
// (`GET /v1/connect/registrars/{registrar_credential_id}`). The
// `credentials` payload is never returned by the API and is therefore not
// exposed by this data source.
type RegistrarCredentialDataSource struct {
	client *opusdns.Client
}

// RegistrarCredentialDataSourceModel is the state shape for the singular datasource.
type RegistrarCredentialDataSourceModel struct {
	ID                    types.String `tfsdk:"id"`
	RegistrarCredentialID types.String `tfsdk:"registrar_credential_id"`
	OrganizationID        types.String `tfsdk:"organization_id"`
	Name                  types.String `tfsdk:"name"`
	Registrar             types.String `tfsdk:"registrar"`
	CreatedOn             types.String `tfsdk:"created_on"`
	UpdatedOn             types.String `tfsdk:"updated_on"`
}

// NewRegistrarCredentialDataSource returns a new RegistrarCredentialDataSource.
func NewRegistrarCredentialDataSource() datasource.DataSource {
	return &RegistrarCredentialDataSource{}
}

func (d *RegistrarCredentialDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registrar_credential"
}

func (d *RegistrarCredentialDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads metadata for a single registrar credential (`GET /v1/connect/registrars/{registrar_credential_id}`). " +
			"The credential payload itself is never returned by the API and is therefore not exposed here.",
		Attributes: map[string]schema.Attribute{
			"id":                      schema.StringAttribute{Computed: true, MarkdownDescription: "Mirrors `registrar_credential_id`."},
			"registrar_credential_id": schema.StringAttribute{Required: true, MarkdownDescription: "The registrar credential id."},
			"organization_id":         schema.StringAttribute{Computed: true},
			"name":                    schema.StringAttribute{Computed: true},
			"registrar":               schema.StringAttribute{Computed: true, MarkdownDescription: "The third-party registrar."},
			"created_on":              schema.StringAttribute{Computed: true},
			"updated_on":              schema.StringAttribute{Computed: true},
		},
	}
}

func (d *RegistrarCredentialDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RegistrarCredentialDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RegistrarCredentialDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cred, err := rawGetRegistrarCredential(ctx, d.client, data.RegistrarCredentialID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading registrar credential", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(cred.RegistrarCredentialID)
	data.RegistrarCredentialID = types.StringValue(cred.RegistrarCredentialID)
	data.OrganizationID = types.StringValue(cred.OrganizationID)
	data.Name = types.StringValue(cred.Name)
	data.Registrar = types.StringValue(cred.Registrar)
	data.CreatedOn = types.StringValue(cred.CreatedOn.Format(time.RFC3339))
	data.UpdatedOn = types.StringValue(cred.UpdatedOn.Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
