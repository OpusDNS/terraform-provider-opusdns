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

// Ensure EventDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &EventDataSource{}

// EventDataSource fetches a single audit event by ID via
// `GET /v1/events/{event_id}`. The `event_data` payload is structurally
// arbitrary so it is exposed as a JSON-encoded string that callers can
// decode with Terraform's `jsondecode()` function as needed.
type EventDataSource struct {
	client *opusdns.Client
}

type EventDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	EventID        types.String `tfsdk:"event_id"`
	Type           types.String `tfsdk:"type"`
	Subtype        types.String `tfsdk:"subtype"`
	ObjectType     types.String `tfsdk:"object_type"`
	ObjectID       types.String `tfsdk:"object_id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	UserID         types.String `tfsdk:"user_id"`
	IPAddress      types.String `tfsdk:"ip_address"`
	UserAgent      types.String `tfsdk:"user_agent"`
	Source         types.String `tfsdk:"source"`
	CreatedOn      types.String `tfsdk:"created_on"`
	EventDataJSON  types.String `tfsdk:"event_data_json"`
}

func NewEventDataSource() datasource.DataSource {
	return &EventDataSource{}
}

func (d *EventDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_event"
}

func (d *EventDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	// Re-use the shared attribute map, but flip event_id to Required and
	// add the synthetic top-level `id` for Terraform.
	attrs := eventAttributeSchema(true)
	attrs["event_id"] = schema.StringAttribute{
		Required:            true,
		MarkdownDescription: "ID of the event to fetch.",
	}
	attrs["id"] = schema.StringAttribute{
		Computed:            true,
		MarkdownDescription: "Same value as `event_id` — present so Terraform has a synthetic identifier.",
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single audit event by ID via `GET /v1/events/{event_id}`. " +
			"The dynamic `event_data` payload is exposed as a JSON string; decode it with " +
			"`jsondecode()` to access specific fields.",
		Attributes: attrs,
	}
}

func (d *EventDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := configureClientFromDataSourceProviderData(req.ProviderData, resp.Diagnostics.AddError); ok {
		d.client = c
	}
}

func (d *EventDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EventDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.EventID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError(
			"Invalid event_id",
			"The `event_id` attribute must be a non-empty string.",
		)
		return
	}

	event, err := d.client.Events.GetEvent(ctx, models.EventID(id))
	if err != nil {
		resp.Diagnostics.AddError("Error fetching event", formatAPIError(err))
		return
	}
	if event == nil {
		resp.Diagnostics.AddError(
			"Empty event response",
			fmt.Sprintf("The /v1/events/%s endpoint returned an empty result.", id),
		)
		return
	}

	obj, diags := eventToObject(*event)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Unpack the object into the flat model. ObjectValue's underlying map
	// gives us a single source-of-truth for value construction.
	attrs := obj.Attributes()
	data.ID = types.StringValue(id)
	data.EventID = attrs["event_id"].(types.String)
	data.Type = attrs["type"].(types.String)
	data.Subtype = attrs["subtype"].(types.String)
	data.ObjectType = attrs["object_type"].(types.String)
	data.ObjectID = attrs["object_id"].(types.String)
	data.OrganizationID = attrs["organization_id"].(types.String)
	data.UserID = attrs["user_id"].(types.String)
	data.IPAddress = attrs["ip_address"].(types.String)
	data.UserAgent = attrs["user_agent"].(types.String)
	data.Source = attrs["source"].(types.String)
	data.CreatedOn = attrs["created_on"].(types.String)
	data.EventDataJSON = attrs["event_data_json"].(types.String)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
