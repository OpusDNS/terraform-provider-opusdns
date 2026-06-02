package provider

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// eventAttrTypes is the shared shape used by both the singular `opusdns_event`
// data source and each element of the `opusdns_events` list. Centralising
// this keeps schema definitions and value construction in lock-step.
var eventAttrTypes = map[string]attr.Type{
	"event_id":        types.StringType,
	"type":            types.StringType,
	"subtype":         types.StringType,
	"object_type":     types.StringType,
	"object_id":       types.StringType,
	"organization_id": types.StringType,
	"user_id":         types.StringType,
	"ip_address":      types.StringType,
	"user_agent":      types.StringType,
	"source":          types.StringType,
	"created_on":      types.StringType,
	// event_data is structurally arbitrary; surface it as a JSON-encoded string
	// to avoid forcing every caller to declare a complete schema for the
	// per-event payload.
	"event_data_json": types.StringType,
}

// eventAttributeSchema returns the per-event nested attribute schema used by
// both `opusdns_event` (flat scalars) and each element in `opusdns_events`.
func eventAttributeSchema(computed bool) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"event_id":        schema.StringAttribute{Computed: true, MarkdownDescription: "Unique event identifier."},
		"type":            schema.StringAttribute{Computed: computed, MarkdownDescription: "Event type (e.g. `DOMAIN_CREATE`, `ZONE_UPDATE`, `NOTIFICATION`)."},
		"subtype":         schema.StringAttribute{Computed: computed, MarkdownDescription: "Optional event subtype, or empty when unset."},
		"object_type":     schema.StringAttribute{Computed: computed, MarkdownDescription: "Type of object the event relates to (e.g. `DOMAIN`, `ZONE`, `CONTACT`)."},
		"object_id":       schema.StringAttribute{Computed: computed, MarkdownDescription: "ID of the related object, or empty if unset."},
		"organization_id": schema.StringAttribute{Computed: computed, MarkdownDescription: "Organization the event belongs to, or empty if unset."},
		"user_id":         schema.StringAttribute{Computed: computed, MarkdownDescription: "User who triggered the event, or empty if system-generated."},
		"ip_address":      schema.StringAttribute{Computed: computed, MarkdownDescription: "Client IP that triggered the event, or empty if unset."},
		"user_agent":      schema.StringAttribute{Computed: computed, MarkdownDescription: "Client user-agent string, or empty if unset."},
		"source":          schema.StringAttribute{Computed: computed, MarkdownDescription: "Event source (`api`, `dashboard`, `system`, etc.). Empty if unset."},
		"created_on":      schema.StringAttribute{Computed: computed, MarkdownDescription: "RFC3339 timestamp of when the event was recorded. Empty if unset."},
		"event_data_json": schema.StringAttribute{Computed: computed, MarkdownDescription: "JSON-encoded `event_data` payload. Decode with `jsondecode()` to access fields."},
	}
}

// eventToObject converts a single SDK Event into a Terraform Object value
// matching eventAttrTypes. Any JSON-marshal failure on the dynamic
// event_data payload is surfaced as a diag so callers see a clean error
// rather than a panic.
func eventToObject(e models.Event) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	dataJSON := "{}"
	if e.EventData != nil {
		b, err := json.Marshal(e.EventData)
		if err != nil {
			diags.AddError("Error marshalling event_data", err.Error())
			return types.ObjectNull(eventAttrTypes), diags
		}
		dataJSON = string(b)
	}
	subtype := ""
	if e.Subtype != nil {
		subtype = string(*e.Subtype)
	}
	objectID := ""
	if e.ObjectID != nil {
		objectID = *e.ObjectID
	}
	orgID := ""
	if e.OrganizationID != nil {
		orgID = string(*e.OrganizationID)
	}
	userID := ""
	if e.UserID != nil {
		userID = string(*e.UserID)
	}
	ipAddr := ""
	if e.IPAddress != nil {
		ipAddr = *e.IPAddress
	}
	userAgent := ""
	if e.UserAgent != nil {
		userAgent = *e.UserAgent
	}
	source := ""
	if e.Source != nil {
		source = *e.Source
	}
	createdOn := ""
	if e.CreatedOn != nil {
		createdOn = e.CreatedOn.Format("2006-01-02T15:04:05Z07:00")
	}
	obj, d := types.ObjectValue(eventAttrTypes, map[string]attr.Value{
		"event_id":        types.StringValue(string(e.EventID)),
		"type":            types.StringValue(string(e.Type)),
		"subtype":         types.StringValue(subtype),
		"object_type":     types.StringValue(string(e.ObjectType)),
		"object_id":       types.StringValue(objectID),
		"organization_id": types.StringValue(orgID),
		"user_id":         types.StringValue(userID),
		"ip_address":      types.StringValue(ipAddr),
		"user_agent":      types.StringValue(userAgent),
		"source":          types.StringValue(source),
		"created_on":      types.StringValue(createdOn),
		"event_data_json": types.StringValue(dataJSON),
	})
	diags.Append(d...)
	return obj, diags
}

// configureClientFromDataSourceProviderData centralises the boilerplate
// every data source uses to retrieve the shared `*opusdns.Client`. The
// existing data sources duplicate this; defined here as a small helper so
// the new event data sources stay terse.
func configureClientFromDataSourceProviderData(providerData any, addError func(string, string)) (*opusdns.Client, bool) {
	if providerData == nil {
		return nil, false
	}
	client, ok := providerData.(*opusdns.Client)
	if !ok {
		addError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T.", providerData))
		return nil, false
	}
	return client, true
}
