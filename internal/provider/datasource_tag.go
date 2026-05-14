package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure TagDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &TagDataSource{}

// TagDataSource reads a single tag by id via `GET /v1/tags/{tag_id}`.
type TagDataSource struct {
	client *opusdns.Client
}

// TagDataSourceModel is the state shape for the singular tag data source.
type TagDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	TagID       types.String `tfsdk:"tag_id"`
	Label       types.String `tfsdk:"label"`
	Type        types.String `tfsdk:"type"`
	Color       types.String `tfsdk:"color"`
	Description types.String `tfsdk:"description"`
	ObjectCount types.Int64  `tfsdk:"object_count"`
	CreatedOn   types.String `tfsdk:"created_on"`
	UpdatedOn   types.String `tfsdk:"updated_on"`
}

// NewTagDataSource returns a new TagDataSource.
func NewTagDataSource() datasource.DataSource {
	return &TagDataSource{}
}

func (d *TagDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag"
}

func (d *TagDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single tag by id (`GET /v1/tags/{tag_id}`).",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true, MarkdownDescription: "Mirrors `tag_id`."},
			"tag_id":       schema.StringAttribute{Required: true, MarkdownDescription: "The tag id (e.g. `tag_01j...`)."},
			"label":        schema.StringAttribute{Computed: true},
			"type":         schema.StringAttribute{Computed: true, MarkdownDescription: "One of `DOMAIN`, `CONTACT`, `ZONE`."},
			"color":        schema.StringAttribute{Computed: true},
			"description":  schema.StringAttribute{Computed: true},
			"object_count": schema.Int64Attribute{Computed: true},
			"created_on":   schema.StringAttribute{Computed: true},
			"updated_on":   schema.StringAttribute{Computed: true},
		},
	}
}

func (d *TagDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TagDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TagDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tag, err := d.client.Tags.GetTag(ctx, models.TagID(data.TagID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading tag", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(string(tag.TagID))
	data.TagID = types.StringValue(string(tag.TagID))
	data.Label = types.StringValue(tag.Label)
	data.Type = types.StringValue(string(tag.Type))
	data.Color = types.StringValue(string(tag.Color))
	data.Description = stringPtrToValue(tag.Description)
	data.ObjectCount = types.Int64Value(int64(tag.ObjectCount))
	data.CreatedOn = types.StringValue(tag.CreatedOn.Format(time.RFC3339))
	data.UpdatedOn = types.StringValue(tag.UpdatedOn.Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
