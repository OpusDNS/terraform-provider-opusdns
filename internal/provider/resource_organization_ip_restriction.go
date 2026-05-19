package provider

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ resource.Resource = &OrganizationIPRestrictionResource{}
var _ resource.ResourceWithImportState = &OrganizationIPRestrictionResource{}

// OrganizationIPRestrictionResource implements `opusdns_organization_ip_restriction`
// backed by `/v1/organizations/ip-restrictions`. Each entry whitelists a
// single IP address or CIDR block for the organization's API access.
//
// `organization_id` is optional on create — the API defaults to the
// authenticated caller's org. It is RequiresReplace because the API has no
// way to move a restriction between orgs.
type OrganizationIPRestrictionResource struct {
	client *opusdns.Client
}

// ipRestrictionAPIResponse mirrors IpRestrictionResponse.
type ipRestrictionAPIResponse struct {
	IPRestrictionID int64      `json:"ip_restriction_id"`
	OrganizationID  string     `json:"organization_id"`
	IPNetwork       string     `json:"ip_network"`
	CreatedOn       time.Time  `json:"created_on"`
	LastUsedOn      *time.Time `json:"last_used_on"`
}

type OrganizationIPRestrictionResourceModel struct {
	ID              types.String `tfsdk:"id"`
	IPRestrictionID types.Int64  `tfsdk:"ip_restriction_id"`
	OrganizationID  types.String `tfsdk:"organization_id"`
	IPNetwork       types.String `tfsdk:"ip_network"`
	CreatedOn       types.String `tfsdk:"created_on"`
	LastUsedOn      types.String `tfsdk:"last_used_on"`
}

func NewOrganizationIPRestrictionResource() resource.Resource {
	return &OrganizationIPRestrictionResource{}
}

func (r *OrganizationIPRestrictionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_ip_restriction"
}

func (r *OrganizationIPRestrictionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	useStateForUnknown := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an organization IP restriction (`/v1/organizations/ip-restrictions`). " +
			"Whitelists a single IP address or CIDR block for the org's API access. " +
			"`organization_id` defaults to the authenticated caller's organization when omitted; " +
			"changing it forces replacement.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mirrors `ip_restriction_id` (as a string).",
				PlanModifiers:       useStateForUnknown,
			},
			"ip_restriction_id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier for the restriction.",
			},
			"organization_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization id (e.g. `organization_01j...`). Defaults to the authenticated caller's org. Immutable; changes force replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ip_network": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "IPv4 or IPv6 address, or CIDR block. Single addresses are normalised " +
					"by the API to `/32` (v4) or `/128` (v6) on read.",
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the restriction was created.",
				PlanModifiers:       useStateForUnknown,
			},
			"last_used_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp of the last request matched against this restriction; null if never used.",
			},
		},
	}
}

func (r *OrganizationIPRestrictionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OrganizationIPRestrictionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationIPRestrictionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]interface{}{
		"ip_network": data.IPNetwork.ValueString(),
	}
	if !data.OrganizationID.IsNull() && !data.OrganizationID.IsUnknown() {
		body["organization_id"] = data.OrganizationID.ValueString()
	}

	out, err := rawCreateIPRestriction(ctx, r.client, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating IP restriction", formatAPIError(err))
		return
	}

	populateIPRestrictionModel(&data, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationIPRestrictionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationIPRestrictionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.IPRestrictionID.ValueInt64()
	if id == 0 {
		resp.Diagnostics.AddError("Invalid IP restriction state", "ip_restriction_id is empty")
		return
	}

	out, err := rawGetIPRestriction(ctx, r.client, id)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading IP restriction", formatAPIError(err))
		return
	}

	populateIPRestrictionModel(&data, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationIPRestrictionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state OrganizationIPRestrictionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.IPRestrictionID.ValueInt64()
	if id == 0 {
		resp.Diagnostics.AddError("Invalid state", "ip_restriction_id is empty")
		return
	}

	var out *ipRestrictionAPIResponse
	if !plan.IPNetwork.Equal(state.IPNetwork) {
		body := map[string]interface{}{"ip_network": plan.IPNetwork.ValueString()}
		updated, err := rawUpdateIPRestriction(ctx, r.client, id, body)
		if err != nil {
			resp.Diagnostics.AddError("Error updating IP restriction", formatAPIError(err))
			return
		}
		out = updated
	} else {
		current, err := rawGetIPRestriction(ctx, r.client, id)
		if err != nil {
			resp.Diagnostics.AddError("Error reading IP restriction", formatAPIError(err))
			return
		}
		out = current
	}

	populateIPRestrictionModel(&plan, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *OrganizationIPRestrictionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrganizationIPRestrictionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := data.IPRestrictionID.ValueInt64()
	if id == 0 {
		return
	}
	if err := rawDeleteIPRestriction(ctx, r.client, id); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting IP restriction", formatAPIError(err))
		}
	}
}

func (r *OrganizationIPRestrictionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import id",
			"opusdns_organization_ip_restriction import expects a numeric ip_restriction_id, got: "+req.ID,
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ip_restriction_id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), strconv.FormatInt(id, 10))...)
}

func populateIPRestrictionModel(data *OrganizationIPRestrictionResourceModel, r *ipRestrictionAPIResponse) {
	data.ID = types.StringValue(strconv.FormatInt(r.IPRestrictionID, 10))
	data.IPRestrictionID = types.Int64Value(r.IPRestrictionID)
	data.OrganizationID = types.StringValue(r.OrganizationID)
	data.IPNetwork = types.StringValue(r.IPNetwork)
	data.CreatedOn = types.StringValue(r.CreatedOn.Format(time.RFC3339))
	if r.LastUsedOn != nil {
		data.LastUsedOn = types.StringValue(r.LastUsedOn.Format(time.RFC3339))
	} else {
		data.LastUsedOn = types.StringNull()
	}
}

// rawCreateIPRestriction wraps POST /v1/organizations/ip-restrictions.
func rawCreateIPRestriction(ctx context.Context, c *opusdns.Client, body map[string]interface{}) (*ipRestrictionAPIResponse, error) {
	path := c.HTTPClient().BuildPath("organizations", "ip-restrictions")
	resp, err := c.HTTPClient().Post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var out ipRestrictionAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawGetIPRestriction wraps GET /v1/organizations/ip-restrictions/{id}.
func rawGetIPRestriction(ctx context.Context, c *opusdns.Client, id int64) (*ipRestrictionAPIResponse, error) {
	path := c.HTTPClient().BuildPath("organizations", "ip-restrictions", strconv.FormatInt(id, 10))
	resp, err := c.HTTPClient().Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var out ipRestrictionAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawUpdateIPRestriction wraps PATCH /v1/organizations/ip-restrictions/{id}.
func rawUpdateIPRestriction(ctx context.Context, c *opusdns.Client, id int64, body map[string]interface{}) (*ipRestrictionAPIResponse, error) {
	path := c.HTTPClient().BuildPath("organizations", "ip-restrictions", strconv.FormatInt(id, 10))
	resp, err := c.HTTPClient().Patch(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var out ipRestrictionAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawDeleteIPRestriction wraps DELETE /v1/organizations/ip-restrictions/{id}.
func rawDeleteIPRestriction(ctx context.Context, c *opusdns.Client, id int64) error {
	path := c.HTTPClient().BuildPath("organizations", "ip-restrictions", strconv.FormatInt(id, 10))
	resp, err := c.HTTPClient().Delete(ctx, path)
	if err != nil {
		return err
	}
	return c.HTTPClient().DecodeResponse(resp, nil)
}

// rawListIPRestrictions wraps GET /v1/organizations/ip-restrictions.
// The endpoint returns a bare array (no pagination wrapper).
func rawListIPRestrictions(ctx context.Context, c *opusdns.Client) ([]ipRestrictionAPIResponse, error) {
	path := c.HTTPClient().BuildPath("organizations", "ip-restrictions")
	resp, err := c.HTTPClient().Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var out []ipRestrictionAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}
