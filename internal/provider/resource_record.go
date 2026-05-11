package provider

import (
	"context"
	"fmt"
	"strings"

	opusdns "github.com/opusdns/opusdns-go-client/opusdns"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &RecordResource{}

type RecordResource struct {
	client *opusdns.Client
}

type RecordResourceModel struct {
	ID       types.String `tfsdk:"id"`
	ZoneName types.String `tfsdk:"zone_name"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	TTL      types.Int64  `tfsdk:"ttl"`
	RData    types.String `tfsdk:"rdata"`
}

func NewRecordResource() resource.Resource {
	return &RecordResource{}
}

func (r *RecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (r *RecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a DNS record in OpusDNS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone_name": schema.StringAttribute{
				Required:    true,
				Description: "The zone name the record belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The record name relative to the zone (e.g., 'www' or '@').",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The record type (A, AAAA, CNAME, MX, TXT, NS, etc.).",
			},
			"ttl": schema.Int64Attribute{
				Required:    true,
				Description: "The TTL of the record in seconds.",
			},
			"rdata": schema.StringAttribute{
				Required:    true,
				Description: "The record data.",
			},
		},
	}
}

func (r *RecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *opusdns.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func recordID(zoneName, name, recType, rdata string) string {
	return fmt.Sprintf("%s:%s:%s:%s", zoneName, name, recType, rdata)
}

func (r *RecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	record := models.Record{
		Name:  plan.Name.ValueString(),
		Type:  models.RRSetType(plan.Type.ValueString()),
		TTL:   int(plan.TTL.ValueInt64()),
		RData: plan.RData.ValueString(),
	}

	if err := r.client.DNS.UpsertRecord(ctx, plan.ZoneName.ValueString(), record); err != nil {
		resp.Diagnostics.AddError("Failed to create record", err.Error())
		return
	}

	plan.ID = types.StringValue(recordID(plan.ZoneName.ValueString(), plan.Name.ValueString(), plan.Type.ValueString(), plan.RData.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := r.client.DNS.GetZone(ctx, state.ZoneName.ValueString())
	if err != nil {
		if opusdns.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read zone for record", err.Error())
		return
	}

	found := false
	for _, rrset := range zone.RRSets {
		if strings.EqualFold(string(rrset.Type), state.Type.ValueString()) && rrset.Name == state.Name.ValueString() {
			for _, rd := range rrset.Records {
				if rd.RData == state.RData.ValueString() {
					state.TTL = types.Int64Value(int64(rrset.TTL))
					found = true
					break
				}
			}
		}
		if found {
			break
		}
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state RecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameChanged := plan.Name.ValueString() != state.Name.ValueString()
	typeChanged := plan.Type.ValueString() != state.Type.ValueString()
	rdataChanged := plan.RData.ValueString() != state.RData.ValueString()

	if nameChanged || typeChanged || rdataChanged {
		oldRecord := models.Record{
			Name:  state.Name.ValueString(),
			Type:  models.RRSetType(state.Type.ValueString()),
			TTL:   int(state.TTL.ValueInt64()),
			RData: state.RData.ValueString(),
		}
		if err := r.client.DNS.DeleteRecord(ctx, state.ZoneName.ValueString(), oldRecord); err != nil {
			if !opusdns.IsNotFoundError(err) {
				resp.Diagnostics.AddError("Failed to delete old record during update", err.Error())
				return
			}
		}
	}

	newRecord := models.Record{
		Name:  plan.Name.ValueString(),
		Type:  models.RRSetType(plan.Type.ValueString()),
		TTL:   int(plan.TTL.ValueInt64()),
		RData: plan.RData.ValueString(),
	}

	if err := r.client.DNS.UpsertRecord(ctx, plan.ZoneName.ValueString(), newRecord); err != nil {
		resp.Diagnostics.AddError("Failed to update record", err.Error())
		return
	}

	plan.ID = types.StringValue(recordID(plan.ZoneName.ValueString(), plan.Name.ValueString(), plan.Type.ValueString(), plan.RData.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := r.client.DNS.GetZone(ctx, state.ZoneName.ValueString())
	if err != nil {
		if opusdns.IsNotFoundError(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to read zone before deleting record", err.Error())
		return
	}

	for _, rrset := range zone.RRSets {
		if strings.EqualFold(string(rrset.Type), state.Type.ValueString()) && rrset.Name == state.Name.ValueString() {
			if rrset.Protected {
				resp.Diagnostics.AddWarning(
					"Record is protected",
					fmt.Sprintf("The record %s %s in zone %s is protected and cannot be deleted.", state.Name.ValueString(), state.Type.ValueString(), state.ZoneName.ValueString()),
				)
				return
			}
			break
		}
	}

	record := models.Record{
		Name:  state.Name.ValueString(),
		Type:  models.RRSetType(state.Type.ValueString()),
		TTL:   int(state.TTL.ValueInt64()),
		RData: state.RData.ValueString(),
	}

	if err := r.client.DNS.DeleteRecord(ctx, state.ZoneName.ValueString(), record); err != nil {
		if !opusdns.IsNotFoundError(err) {
			resp.Diagnostics.AddError("Failed to delete record", err.Error())
		}
	}
}
