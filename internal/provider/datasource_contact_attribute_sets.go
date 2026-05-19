package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ datasource.DataSource = &ContactAttributeSetsDataSource{}

// ContactAttributeSetsDataSource lists contact attribute sets
// (`GET /v1/contacts/attribute-sets`), auto-paginated.
type ContactAttributeSetsDataSource struct {
	client *opusdns.Client
}

type ContactAttributeSetsDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	TLD                  types.String `tfsdk:"tld"`
	Label                types.String `tfsdk:"label"`
	ContactAttributeSets types.List   `tfsdk:"contact_attribute_sets"`
}

var contactAttributeSetItemAttrTypes = map[string]attr.Type{
	"contact_attribute_set_id": types.StringType,
	"organization_id":          types.StringType,
	"label":                    types.StringType,
	"tld":                      types.StringType,
	"attributes":               types.MapType{ElemType: types.StringType},
	"linked_contacts":          types.Int64Type,
	"created_on":               types.StringType,
	"updated_on":               types.StringType,
}

func NewContactAttributeSetsDataSource() datasource.DataSource {
	return &ContactAttributeSetsDataSource{}
}

func (d *ContactAttributeSetsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contact_attribute_sets"
}

func (d *ContactAttributeSetsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists contact attribute sets in the authenticated caller's organization " +
			"(`GET /v1/contacts/attribute-sets`). Optional `tld` and `label` filters narrow the " +
			"server-side query; results are auto-paginated.",
		Attributes: map[string]schema.Attribute{
			"id":    schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier."},
			"tld":   schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by TLD."},
			"label": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by label."},
			"contact_attribute_sets": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching contact attribute sets.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"contact_attribute_set_id": schema.StringAttribute{Computed: true},
						"organization_id":          schema.StringAttribute{Computed: true},
						"label":                    schema.StringAttribute{Computed: true},
						"tld":                      schema.StringAttribute{Computed: true},
						"attributes":               schema.MapAttribute{Computed: true, ElementType: types.StringType},
						"linked_contacts":          schema.Int64Attribute{Computed: true},
						"created_on":               schema.StringAttribute{Computed: true},
						"updated_on":               schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ContactAttributeSetsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ContactAttributeSetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ContactAttributeSetsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sets, err := rawListContactAttributeSets(ctx, d.client, stringValue(data.TLD), stringValue(data.Label))
	if err != nil {
		resp.Diagnostics.AddError("Error listing contact attribute sets", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: contactAttributeSetItemAttrTypes}
	values := make([]attr.Value, len(sets))
	for i := range sets {
		s := &sets[i]
		mv, diags := types.MapValueFrom(ctx, types.StringType, s.Attributes)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		obj, diags := types.ObjectValue(contactAttributeSetItemAttrTypes, map[string]attr.Value{
			"contact_attribute_set_id": types.StringValue(s.ContactAttributeSetID),
			"organization_id":          types.StringValue(s.OrganizationID),
			"label":                    types.StringValue(s.Label),
			"tld":                      types.StringValue(s.TLD),
			"attributes":               mv,
			"linked_contacts":          types.Int64Value(s.LinkedContacts),
			"created_on":               types.StringValue(s.CreatedOn.Format(time.RFC3339)),
			"updated_on":               types.StringValue(s.UpdatedOn.Format(time.RFC3339)),
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

	data.ID = types.StringValue("contact_attribute_sets")
	data.ContactAttributeSets = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
