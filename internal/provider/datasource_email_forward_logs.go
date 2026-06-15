package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure EmailForwardLogsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &EmailForwardLogsDataSource{}

// EmailForwardLogsDataSource reads email-forward delivery logs from
// `GET /v1/archive/email-forward-logs/{email_forward_id}` or, when an
// alias id is provided, `GET /v1/archive/email-forward-logs/aliases/{alias_id}`.
// Exactly one of `email_forward_id` and `email_forward_alias_id` must be set.
type EmailForwardLogsDataSource struct {
	client *opusdns.Client
}

type EmailForwardLogsDataSourceModel struct {
	ID                  types.String `tfsdk:"id"`
	EmailForwardID      types.String `tfsdk:"email_forward_id"`
	EmailForwardAliasID types.String `tfsdk:"email_forward_alias_id"`
	Logs                types.List   `tfsdk:"logs"`
	TotalReturned       types.Int64  `tfsdk:"total_returned"`
}

var emailForwardLogEventAttrTypes = map[string]attr.Type{
	"id":      types.StringType,
	"code":    types.Int64Type,
	"status":  types.StringType,
	"message": types.StringType,
	"server":  types.StringType,
	"local":   types.StringType,
	"created": types.StringType,
}

var emailForwardLogAttrTypes = map[string]attr.Type{
	"log_id":          types.StringType,
	"domain":          types.StringType,
	"sender_email":    types.StringType,
	"sender_name":     types.StringType,
	"recipient_email": types.StringType,
	"recipient_name":  types.StringType,
	"forward_email":   types.StringType,
	"forward_name":    types.StringType,
	"subject":         types.StringType,
	"hostname":        types.StringType,
	"message_id":      types.StringType,
	"transport":       types.StringType,
	"final_status":    types.StringType,
	"created_on":      types.StringType,
	"synced_on":       types.StringType,
	"events":          types.ListType{ElemType: types.ObjectType{AttrTypes: emailForwardLogEventAttrTypes}},
}

func NewEmailForwardLogsDataSource() datasource.DataSource {
	return &EmailForwardLogsDataSource{}
}

func (d *EmailForwardLogsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_email_forward_logs"
}

func (d *EmailForwardLogsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists email-forward delivery logs from the OpusDNS archive. " +
			"Set `email_forward_id` to read `GET /v1/archive/email-forward-logs/{id}`, or " +
			"`email_forward_alias_id` to read " +
			"`GET /v1/archive/email-forward-logs/aliases/{id}`. Exactly one of the two " +
			"must be provided.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Static identifier for this data source.",
			},
			"email_forward_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Email forward id to look up. Mutually exclusive with `email_forward_alias_id`.",
			},
			"email_forward_alias_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Email forward alias id to look up. Mutually exclusive with `email_forward_id`.",
			},
			"total_returned": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of entries returned (length of `logs`).",
			},
			"logs": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Delivery log entries (most recent first).",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"log_id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Upstream log identifier from ImprovMX."},
						"domain":          schema.StringAttribute{Computed: true, MarkdownDescription: "Domain that received the email."},
						"sender_email":    schema.StringAttribute{Computed: true, MarkdownDescription: "Sender email address."},
						"sender_name":     schema.StringAttribute{Computed: true, MarkdownDescription: "Sender display name, or empty if unset."},
						"recipient_email": schema.StringAttribute{Computed: true, MarkdownDescription: "Recipient email address (the alias)."},
						"recipient_name":  schema.StringAttribute{Computed: true, MarkdownDescription: "Recipient display name, or empty if unset."},
						"forward_email":   schema.StringAttribute{Computed: true, MarkdownDescription: "Final destination email address."},
						"forward_name":    schema.StringAttribute{Computed: true, MarkdownDescription: "Forward destination display name, or empty if unset."},
						"subject":         schema.StringAttribute{Computed: true, MarkdownDescription: "Email subject line."},
						"hostname":        schema.StringAttribute{Computed: true, MarkdownDescription: "Hostname of the receiving MTA."},
						"message_id":      schema.StringAttribute{Computed: true, MarkdownDescription: "Email message id."},
						"transport":       schema.StringAttribute{Computed: true, MarkdownDescription: "Transport method used (`mx` or `smtp`)."},
						"final_status":    schema.StringAttribute{Computed: true, MarkdownDescription: "Final delivery status."},
						"created_on":      schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the email was received."},
						"synced_on":       schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the log was synced to OpusDNS."},
						"events": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Per-attempt delivery events (queued, delivered, refused, bounce, ...).",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id":      schema.StringAttribute{Computed: true, MarkdownDescription: "Event identifier."},
									"code":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Event status code."},
									"status":  schema.StringAttribute{Computed: true, MarkdownDescription: "Event status (`QUEUED`, `DELIVERED`, `REFUSED`, `SOFT-BOUNCE`, `HARD-BOUNCE`)."},
									"message": schema.StringAttribute{Computed: true, MarkdownDescription: "Event message."},
									"server":  schema.StringAttribute{Computed: true, MarkdownDescription: "Server that processed the event."},
									"local":   schema.StringAttribute{Computed: true, MarkdownDescription: "Local ImprovMX server that processed the event."},
									"created": schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp of when the event occurred."},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *EmailForwardLogsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := configureClientFromDataSourceProviderData(req.ProviderData, resp.Diagnostics.AddError); ok {
		d.client = c
	}
}

func (d *EmailForwardLogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EmailForwardLogsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	forwardID := data.EmailForwardID.ValueString()
	aliasID := data.EmailForwardAliasID.ValueString()
	if forwardID == "" && aliasID == "" {
		resp.Diagnostics.AddError(
			"Missing email forward identifier",
			"Set either `email_forward_id` or `email_forward_alias_id` to look up logs.",
		)
		return
	}
	if forwardID != "" && aliasID != "" {
		resp.Diagnostics.AddError(
			"Conflicting email forward identifiers",
			"Set only one of `email_forward_id` or `email_forward_alias_id`, not both.",
		)
		return
	}

	var (
		result *models.EmailForwardLogListResponse
		err    error
	)
	switch {
	case forwardID != "":
		result, err = d.client.Events.ListEmailForwardLogs(ctx, models.EmailForwardID(forwardID))
	default:
		result, err = d.client.Events.ListEmailForwardLogsByAlias(ctx, models.EmailForwardAliasID(aliasID))
	}
	if err != nil {
		resp.Diagnostics.AddError("Error listing email forward logs", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: emailForwardLogAttrTypes}
	values := make([]attr.Value, len(result.Results))
	for i, l := range result.Results {
		obj, d := emailForwardLogToObject(l)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		values[i] = obj
	}
	list, ld := types.ListValue(objType, values)
	resp.Diagnostics.Append(ld...)
	if resp.Diagnostics.HasError() {
		return
	}

	idLabel := forwardID
	if idLabel == "" {
		idLabel = "alias:" + aliasID
	}
	data.ID = types.StringValue(idLabel)
	data.Logs = list
	data.TotalReturned = types.Int64Value(int64(len(result.Results)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func emailForwardLogToObject(l models.EmailForwardLog) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	eventObjType := types.ObjectType{AttrTypes: emailForwardLogEventAttrTypes}
	eventValues := make([]attr.Value, len(l.Events))
	for i, ev := range l.Events {
		evObj, d := types.ObjectValue(emailForwardLogEventAttrTypes, map[string]attr.Value{
			"id":      types.StringValue(ev.ID),
			"code":    types.Int64Value(int64(ev.Code)),
			"status":  types.StringValue(ev.Status),
			"message": types.StringValue(ev.Message),
			"server":  types.StringValue(ev.Server),
			"local":   types.StringValue(ev.Local),
			"created": types.StringValue(ev.Created.Format("2006-01-02T15:04:05Z07:00")),
		})
		diags.Append(d...)
		if diags.HasError() {
			return types.ObjectNull(emailForwardLogAttrTypes), diags
		}
		eventValues[i] = evObj
	}
	eventsList, ld := types.ListValue(eventObjType, eventValues)
	diags.Append(ld...)
	if diags.HasError() {
		return types.ObjectNull(emailForwardLogAttrTypes), diags
	}

	senderName := ""
	if l.SenderName != nil {
		senderName = *l.SenderName
	}
	recipientName := ""
	if l.RecipientName != nil {
		recipientName = *l.RecipientName
	}
	forwardName := ""
	if l.ForwardName != nil {
		forwardName = *l.ForwardName
	}

	obj, od := types.ObjectValue(emailForwardLogAttrTypes, map[string]attr.Value{
		"log_id":          types.StringValue(l.LogID),
		"domain":          types.StringValue(l.Domain),
		"sender_email":    types.StringValue(l.SenderEmail),
		"sender_name":     types.StringValue(senderName),
		"recipient_email": types.StringValue(l.RecipientEmail),
		"recipient_name":  types.StringValue(recipientName),
		"forward_email":   types.StringValue(l.ForwardEmail),
		"forward_name":    types.StringValue(forwardName),
		"subject":         types.StringValue(l.Subject),
		"hostname":        types.StringValue(l.Hostname),
		"message_id":      types.StringValue(l.MessageID),
		"transport":       types.StringValue(l.Transport),
		"final_status":    types.StringValue(string(l.FinalStatus)),
		"created_on":      types.StringValue(l.CreatedOn.Format("2006-01-02T15:04:05Z07:00")),
		"synced_on":       types.StringValue(l.SyncedOn.Format("2006-01-02T15:04:05Z07:00")),
		"events":          eventsList,
	})
	diags.Append(od...)
	return obj, diags
}
