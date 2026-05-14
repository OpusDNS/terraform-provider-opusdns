package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure TagsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &TagsDataSource{}

// TagsDataSource lists tags via `GET /v1/tags`, auto-paginated by the SDK.
type TagsDataSource struct {
	client *opusdns.Client
}

// TagsDataSourceModel is the state shape for the tags list data source.
type TagsDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Search   types.String `tfsdk:"search"`
	TagTypes types.List   `tfsdk:"tag_types"`
	Tags     types.List   `tfsdk:"tags"`
}

var tagItemAttrTypes = map[string]attr.Type{
	"tag_id":       types.StringType,
	"label":        types.StringType,
	"type":         types.StringType,
	"color":        types.StringType,
	"description":  types.StringType,
	"object_count": types.Int64Type,
	"created_on":   types.StringType,
	"updated_on":   types.StringType,
}

// NewTagsDataSource returns a new TagsDataSource.
func NewTagsDataSource() datasource.DataSource {
	return &TagsDataSource{}
}

func (d *TagsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tags"
}

func (d *TagsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists tags within the authenticated caller's organization (`GET /v1/tags`). " +
			"Results are auto-paginated by the SDK; optional filters narrow the server-side query.",
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"search": schema.StringAttribute{Optional: true, MarkdownDescription: "Free-text search over tag labels and descriptions."},
			"tag_types": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Restrict results to the given tag types. Each entry must be one of `DOMAIN`, `CONTACT`, `ZONE`.",
			},

			"tags": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching tags.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"tag_id":       schema.StringAttribute{Computed: true},
						"label":        schema.StringAttribute{Computed: true},
						"type":         schema.StringAttribute{Computed: true},
						"color":        schema.StringAttribute{Computed: true},
						"description":  schema.StringAttribute{Computed: true},
						"object_count": schema.Int64Attribute{Computed: true},
						"created_on":   schema.StringAttribute{Computed: true},
						"updated_on":   schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *TagsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TagsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TagsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &models.ListTagsOptions{
		Search: stringValue(data.Search),
	}
	if !data.TagTypes.IsNull() && !data.TagTypes.IsUnknown() {
		var raw []string
		resp.Diagnostics.Append(data.TagTypes.ElementsAs(ctx, &raw, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		opts.TagTypes = make([]models.TagType, 0, len(raw))
		for _, t := range raw {
			opts.TagTypes = append(opts.TagTypes, models.TagType(t))
		}
	}

	tags, err := d.client.Tags.ListTags(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing tags", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: tagItemAttrTypes}
	values := make([]attr.Value, len(tags))
	for i := range tags {
		t := &tags[i]
		obj, diags := types.ObjectValue(tagItemAttrTypes, map[string]attr.Value{
			"tag_id":       types.StringValue(string(t.TagID)),
			"label":        types.StringValue(t.Label),
			"type":         types.StringValue(string(t.Type)),
			"color":        types.StringValue(string(t.Color)),
			"description":  stringPtrToValue(t.Description),
			"object_count": types.Int64Value(int64(t.ObjectCount)),
			"created_on":   types.StringValue(t.CreatedOn.Format(time.RFC3339)),
			"updated_on":   types.StringValue(t.UpdatedOn.Format(time.RFC3339)),
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

	data.ID = types.StringValue("tags")
	data.Tags = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
