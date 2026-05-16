package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure ParkingResource satisfies the resource.Resource interface.
var _ resource.Resource = &ParkingResource{}
var _ resource.ResourceWithImportState = &ParkingResource{}

// ParkingResource implements `opusdns_parking` backed by `/v1/parking`.
//
// A parking entry attaches an ad-serving "parked" placeholder page to a
// domain owned by the authenticated organization. The OpusDNS SDK at v1.0.9
// does not expose a typed Parking service, so this resource is wired
// directly to the SDK's low-level HTTPClient via the helpers in
// parking_helpers.go. See api/api/parking/v1_routes.py and
// common/schemas/parking.py for the API contract.
//
// `domain` is RequiresReplace because the API has no rename endpoint —
// changing the domain requires a delete + create. `enabled` is mutable and
// is toggled via PATCH `/enable` and `/disable` subresources.
//
// Note: the parking endpoints require that the organization has accepted
// the parking program agreement (the `require_parking_signup` dependency
// on the API routes). This resource does not attempt to perform that
// signup; the calling organization must accept the agreement out of band
// before any parking entries can be created.
type ParkingResource struct {
	client *opusdns.Client
}

// ParkingResourceModel mirrors ParkingResponse plus the create-time
// `domain` (immutable post-create) and `enabled` toggle.
type ParkingResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ParkingID        types.String `tfsdk:"parking_id"`
	Domain           types.String `tfsdk:"domain"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	ComplianceStatus types.String `tfsdk:"compliance_status"`
	ContentLanguage  types.String `tfsdk:"content_language"`
	Note             types.String `tfsdk:"note"`
	ContentURL       types.String `tfsdk:"content_url"`
	CreatedOn        types.String `tfsdk:"created_on"`
	UpdatedOn        types.String `tfsdk:"updated_on"`
}

// NewParkingResource returns a new ParkingResource.
func NewParkingResource() resource.Resource {
	return &ParkingResource{}
}

func (r *ParkingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_parking"
}

func (r *ParkingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	useStateForUnknown := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a parking entry in OpusDNS via `/v1/parking`. A parking entry attaches " +
			"an ad-serving placeholder page to a domain owned by the authenticated organization. " +
			"The organization must have accepted the parking program agreement (see " +
			"`/v1/parking/signup`) before parking entries can be created.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The parking ID (mirrors `parking_id`).",
				PlanModifiers:       useStateForUnknown,
			},
			"parking_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the parking entry (e.g. `parking_01j...`).",
				PlanModifiers:       useStateForUnknown,
			},
			"domain": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The domain name to park (must exist in the authenticated organization). Immutable after create.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether parking is enabled for this domain. Defaults to `false` on create. Toggled via `PATCH /v1/parking/{ref}/enable` and `/disable`.",
			},
			"compliance_status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server-assigned compliance status of the parking ad. One of `preparing`, `pending`, `approved`, `disapproved`, `expired`, or null when not yet evaluated.",
			},
			"content_language": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Primary language code for the ad content (server-assigned).",
			},
			"note": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Additional notes about the parking ad (server-assigned).",
			},
			"content_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The content URL for approved parking ads (server-assigned).",
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the parking entry was created.",
				PlanModifiers:       useStateForUnknown,
			},
			"updated_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the parking entry was last updated.",
			},
		},
	}
}

func (r *ParkingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *ParkingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ParkingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]interface{}{
		"domain": data.Domain.ValueString(),
	}
	// `enabled` defaults to false server-side; only send it when explicitly
	// configured so the server-side default applies for unset values.
	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() {
		body["enabled"] = data.Enabled.ValueBool()
	}

	parking, err := rawCreateParking(ctx, r.client, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating parking entry", formatAPIError(err))
		return
	}

	applyParkingToModel(&data, parking, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ParkingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ParkingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parkingID := data.ParkingID.ValueString()
	if parkingID == "" {
		resp.Diagnostics.AddError(
			"Invalid parking state",
			"The opusdns_parking resource has an empty `parking_id` in state, which prevents reading it from the API. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	parking, err := rawGetParking(ctx, r.client, parkingID)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading parking entry", formatAPIError(err))
		return
	}

	applyParkingToModel(&data, parking, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ParkingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ParkingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parkingID := state.ParkingID.ValueString()
	if parkingID == "" {
		resp.Diagnostics.AddError(
			"Invalid parking state",
			"The opusdns_parking resource has an empty `parking_id` in state, which prevents updating it. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	var parking *parkingAPIResponse
	if !plan.Enabled.Equal(state.Enabled) && !plan.Enabled.IsUnknown() && !plan.Enabled.IsNull() {
		var err error
		if plan.Enabled.ValueBool() {
			parking, err = rawEnableParking(ctx, r.client, parkingID)
		} else {
			parking, err = rawDisableParking(ctx, r.client, parkingID)
		}
		if err != nil {
			resp.Diagnostics.AddError("Error updating parking entry", formatAPIError(err))
			return
		}
	} else {
		current, err := rawGetParking(ctx, r.client, parkingID)
		if err != nil {
			resp.Diagnostics.AddError("Error reading parking entry", formatAPIError(err))
			return
		}
		parking = current
	}

	applyParkingToModel(&plan, parking, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ParkingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ParkingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parkingID := data.ParkingID.ValueString()
	if parkingID == "" {
		resp.Diagnostics.AddError(
			"Invalid parking state",
			"The opusdns_parking resource has an empty `parking_id` in state, which prevents deletion via the API. "+
				"Remove the resource from state with `terraform state rm` and, if the parking entry still exists at OpusDNS, delete it manually or re-import then destroy.",
		)
		return
	}

	if err := rawDeleteParking(ctx, r.client, parkingID); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting parking entry", formatAPIError(err))
		}
	}
}

func (r *ParkingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by parking_id or domain. The API accepts either as the path
	// parameter; Read hydrates the full state.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("parking_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
