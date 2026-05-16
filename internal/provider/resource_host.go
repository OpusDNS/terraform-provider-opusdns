package provider

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure HostResource satisfies the resource.Resource interface.
var _ resource.Resource = &HostResource{}
var _ resource.ResourceWithImportState = &HostResource{}

// HostResource implements `opusdns_host` backed by `/v1/hosts`. A host object
// represents a registry "name server host" — an FQDN with one or more IP
// addresses — that can be used as a glue record for domains whose nameservers
// are subordinate to those domains (e.g. `ns1.example.com` as a nameserver
// for `example.com`).
//
// The OpusDNS SDK at v1.0.9 does not expose a typed Hosts service, so this
// resource is implemented directly against the SDK's low-level HTTPClient
// (which preserves the provider's bearer-token transport). See
// api/api/host/v1_routes.py and common/models/domain/host.py for the
// underlying API contract.
type HostResource struct {
	client *opusdns.Client
}

// HostResourceModel mirrors HostResponse plus the create-time `hostname`
// (immutable post-create, since the API has no rename operation).
type HostResourceModel struct {
	ID          types.String `tfsdk:"id"`
	HostID      types.String `tfsdk:"host_id"`
	Hostname    types.String `tfsdk:"hostname"`
	IPAddresses types.List   `tfsdk:"ip_addresses"`
	CreatedOn   types.String `tfsdk:"created_on"`
	UpdatedOn   types.String `tfsdk:"updated_on"`
}

// hostAPIResponse mirrors common/models/domain/host.py:HostResponse.
type hostAPIResponse struct {
	HostID      string    `json:"host_id"`
	Hostname    string    `json:"hostname"`
	IPAddresses []string  `json:"ip_addresses"`
	CreatedOn   time.Time `json:"created_on"`
	UpdatedOn   time.Time `json:"updated_on"`
}

// NewHostResource returns a new HostResource.
func NewHostResource() resource.Resource {
	return &HostResource{}
}

func (r *HostResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

func (r *HostResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	useStateForUnknown := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a host object (glue record) in OpusDNS via `/v1/hosts`. " +
			"Host objects are subordinate to a parent domain in the same organization and bind a hostname " +
			"to one or more IP addresses. Used as glue when the nameserver for a zone is itself a subdomain " +
			"of that zone (e.g. `ns1.example.com` serving `example.com`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The host ID (mirrors `host_id`).",
				PlanModifiers:       useStateForUnknown,
			},
			"host_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the host object (e.g. `host_01j...`).",
				PlanModifiers:       useStateForUnknown,
			},
			"hostname": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Fully-qualified hostname of the host object (e.g. `ns1.example.com`). Immutable after create.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ip_addresses": schema.ListAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "List of IPv4 or IPv6 addresses for the host object. At least one address is required.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the host was created.",
				PlanModifiers:       useStateForUnknown,
			},
			"updated_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the host was last updated.",
			},
		},
	}
}

func (r *HostResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *HostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HostResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ips, diags := listToStringSlice(ctx, data.IPAddresses)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]interface{}{
		"hostname":     data.Hostname.ValueString(),
		"ip_addresses": ips,
	}

	host, err := rawCreateHost(ctx, r.client, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating host", formatAPIError(err))
		return
	}

	populateHostModel(ctx, &data, host, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HostResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostID := data.HostID.ValueString()
	if hostID == "" {
		resp.Diagnostics.AddError(
			"Invalid host state",
			"The opusdns_host resource has an empty `host_id` in state, which prevents reading it from the API. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	host, err := rawGetHost(ctx, r.client, hostID)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading host", formatAPIError(err))
		return
	}

	populateHostModel(ctx, &data, host, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state HostResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostID := state.HostID.ValueString()
	if hostID == "" {
		resp.Diagnostics.AddError(
			"Invalid host state",
			"The opusdns_host resource has an empty `host_id` in state, which prevents updating it. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	planIPs, diags := listToStringSlice(ctx, plan.IPAddresses)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	stateIPs, diags := listToStringSlice(ctx, state.IPAddresses)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var host *hostAPIResponse
	if !ipSetsEqual(planIPs, stateIPs) {
		body := map[string]interface{}{"ip_addresses": planIPs}
		updated, err := rawUpdateHost(ctx, r.client, hostID, body)
		if err != nil {
			resp.Diagnostics.AddError("Error updating host", formatAPIError(err))
			return
		}
		host = updated
	} else {
		current, err := rawGetHost(ctx, r.client, hostID)
		if err != nil {
			resp.Diagnostics.AddError("Error reading host", formatAPIError(err))
			return
		}
		host = current
	}

	populateHostModel(ctx, &plan, host, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *HostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HostResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostID := data.HostID.ValueString()
	if hostID == "" {
		resp.Diagnostics.AddError(
			"Invalid host state",
			"The opusdns_host resource has an empty `host_id` in state, which prevents deletion via the API. "+
				"Remove the resource from state with `terraform state rm` and, if the host still exists at OpusDNS, delete it manually or re-import then destroy.",
		)
		return
	}

	if err := rawDeleteHost(ctx, r.client, hostID); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting host", formatAPIError(err))
		}
	}
}

func (r *HostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by host_id or hostname. The API accepts either as the path
	// parameter on subsequent reads. Hydrate full state via Read.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("host_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// populateHostModel copies API response fields onto a HostResourceModel.
func populateHostModel(_ context.Context, data *HostResourceModel, h *hostAPIResponse, diags *diag.Diagnostics) {
	data.ID = types.StringValue(h.HostID)
	data.HostID = types.StringValue(h.HostID)
	data.Hostname = types.StringValue(h.Hostname)

	// Sort IPs for deterministic state ordering so plan-vs-state diffs do not
	// flap when the API returns addresses in a different order than supplied.
	ips := append([]string(nil), h.IPAddresses...)
	sort.Strings(ips)
	values := make([]attr.Value, len(ips))
	for i, ip := range ips {
		values[i] = types.StringValue(ip)
	}
	list, d := types.ListValue(types.StringType, values)
	diags.Append(d...)
	data.IPAddresses = list

	data.CreatedOn = types.StringValue(h.CreatedOn.Format(time.RFC3339))
	data.UpdatedOn = types.StringValue(h.UpdatedOn.Format(time.RFC3339))
}

// ipSetsEqual reports whether two IP lists contain the same addresses,
// independent of input ordering. The API has no canonical order for
// `ip_addresses`, so plan vs state membership is what matters.
func ipSetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := append([]string(nil), a...)
	sb := append([]string(nil), b...)
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}
