package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure OrganizationAttributesDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &OrganizationAttributesDataSource{}

// OrganizationAttributesDataSource exposes the key/value attributes for an
// organization (`GET /v1/organizations/attributes` for the current org or
// `GET /v1/organizations/{organization_id}/attributes` for a specific one).
//
// The SDK helper `Organizations.GetAttributes` ships with the path
// `organizations/attributes/{orgID}` and an `OrganizationAttributesResponse`
// wrapper struct — both wrong against the live API, which uses the path
// shapes above and returns a bare list. This data source bypasses the SDK
// helper and decodes the bare-array response via the raw HTTP client (same
// approach as `datasource_tld_portfolio.go`). Tracked alongside the other
// SDK drifts in `docs/bugs/sdk-organizations-billing-pricing-drift.md`.
type OrganizationAttributesDataSource struct {
	client *opusdns.Client
}

type OrganizationAttributesDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Me             types.Bool   `tfsdk:"me"`
	Keys           types.List   `tfsdk:"keys"`
	Attributes     types.List   `tfsdk:"attributes"`
	Total          types.Int64  `tfsdk:"total"`
}

var organizationAttributeAttrTypes = map[string]attr.Type{
	"organization_attribute_id": types.Int64Type,
	"key":                       types.StringType,
	// value is JSON-shaped; surfaced as an opaque JSON string to avoid
	// forcing every caller to declare a schema for arbitrary payloads.
	"value_json": types.StringType,
	"protected":  types.BoolType,
	"created_on": types.StringType,
	"updated_on": types.StringType,
}

// organizationAttributeWire mirrors the live API
// (api/common/models/account/organization_attribute.py: OrganizationAttributeResponse).
// Kept private to this file because it's the on-the-wire shape, not part
// of the public model.
type organizationAttributeWire struct {
	OrganizationAttributeID int64           `json:"organization_attribute_id"`
	Key                     string          `json:"key"`
	Value                   json.RawMessage `json:"value"`
	Protected               bool            `json:"protected"`
	CreatedOn               *string         `json:"created_on,omitempty"`
	UpdatedOn               *string         `json:"updated_on,omitempty"`
}

func NewOrganizationAttributesDataSource() datasource.DataSource {
	return &OrganizationAttributesDataSource{}
}

func (d *OrganizationAttributesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_attributes"
}

func (d *OrganizationAttributesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the organization-level key/value attributes for the calling " +
			"organization (`GET /v1/organizations/attributes`) or a specific one " +
			"(`GET /v1/organizations/{organization_id}/attributes`). Either set " +
			"`organization_id` to look up a specific org, set `me = true` to use the " +
			"caller's own org, or leave both unset to hit the `/attributes` endpoint " +
			"(equivalent to `me = true`). Use `keys` to narrow the response to a " +
			"specific subset. Attribute values can be arbitrary JSON; surface them as " +
			"strings via `value_json` and decode with `jsondecode()`.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"organization_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Organization id to look up. Mutually exclusive with `me`. Leave both unset to use the caller's org."},
			"me":              schema.BoolAttribute{Optional: true, MarkdownDescription: "When true, resolve the caller's organization id via `/v1/users/me` and use that. Mutually exclusive with `organization_id`."},
			"keys": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Optional list of attribute keys to return. Multiple values are sent as repeated `keys` query parameters.",
			},
			"total": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of attributes returned (length of `attributes`).",
			},
			"attributes": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Attributes matching the request.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"organization_attribute_id": schema.Int64Attribute{Computed: true, MarkdownDescription: "Internal numeric id of the attribute row."},
						"key":                       schema.StringAttribute{Computed: true, MarkdownDescription: "Attribute key (e.g. `hubspot_organization_id`)."},
						"value_json":                schema.StringAttribute{Computed: true, MarkdownDescription: "Attribute value encoded as JSON (use `jsondecode()`). `null` when the attribute has no value set."},
						"protected":                 schema.BoolAttribute{Computed: true, MarkdownDescription: "When true, the attribute is protected and cannot be modified by users."},
						"created_on":                schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the attribute was created, or empty if unset."},
						"updated_on":                schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the attribute was last updated, or empty if unset."},
					},
				},
			},
		},
	}
}

func (d *OrganizationAttributesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := configureClientFromDataSourceProviderData(req.ProviderData, resp.Diagnostics.AddError); ok {
		d.client = c
	}
}

func (d *OrganizationAttributesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationAttributesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgID, idLabel, diags := resolveOrgSelector(ctx, d.client, data.OrganizationID, data.Me)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var path string
	if orgID == "" {
		// Caller-org variant: GET /v1/organizations/attributes
		path = d.client.HTTPClient().BuildPath("organizations", "attributes")
	} else {
		// Specific-org variant: GET /v1/organizations/{organization_id}/attributes
		path = d.client.HTTPClient().BuildPath("organizations", string(orgID), "attributes")
	}

	query := url.Values{}
	if !data.Keys.IsNull() && !data.Keys.IsUnknown() {
		keys, kd := stringListValueToStrings(ctx, data.Keys)
		resp.Diagnostics.Append(kd...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, k := range keys {
			query.Add("keys", k)
		}
	}

	httpResp, err := d.client.HTTPClient().Get(ctx, path, query)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching organization attributes", formatAPIError(err))
		return
	}
	var entries []organizationAttributeWire
	if err := d.client.HTTPClient().DecodeResponse(httpResp, &entries); err != nil {
		var raw json.RawMessage
		_ = d.client.HTTPClient().DecodeResponse(httpResp, &raw)
		resp.Diagnostics.AddError(
			"Error decoding organization attributes response",
			fmt.Sprintf("expected a JSON array of attribute objects: %s\nraw body: %s",
				err.Error(), string(raw)),
		)
		return
	}

	objType := types.ObjectType{AttrTypes: organizationAttributeAttrTypes}
	values := make([]attr.Value, len(entries))
	for i, e := range entries {
		valueJSON := "null"
		if len(e.Value) > 0 {
			valueJSON = string(e.Value)
		}
		createdOn := ""
		if e.CreatedOn != nil {
			createdOn = *e.CreatedOn
		}
		updatedOn := ""
		if e.UpdatedOn != nil {
			updatedOn = *e.UpdatedOn
		}
		obj, d := types.ObjectValue(organizationAttributeAttrTypes, map[string]attr.Value{
			"organization_attribute_id": types.Int64Value(e.OrganizationAttributeID),
			"key":                       types.StringValue(e.Key),
			"value_json":                types.StringValue(valueJSON),
			"protected":                 types.BoolValue(e.Protected),
			"created_on":                types.StringValue(createdOn),
			"updated_on":                types.StringValue(updatedOn),
		})
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

	data.ID = types.StringValue(idLabel)
	data.Attributes = list
	data.Total = types.Int64Value(int64(len(entries)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// resolveOrgSelector turns the (organization_id, me) pair into an
// `OrganizationID` plus a printable identifier suitable for the data
// source's `id` field. Returns ("", "current") to indicate "use the
// caller-org variant of the endpoint" (i.e. omit org id from the path).
//
// Rules:
//   - both set -> error.
//   - me=true  -> resolve current user, return (uid_of_user.org, that-id-as-string).
//   - org_id   -> return (id, id).
//   - neither  -> return ("", "current") so callers can use the
//     caller-org-implied endpoint variant.
func resolveOrgSelector(ctx context.Context, client *opusdns.Client, organizationID types.String, me types.Bool) (models.OrganizationID, string, diag.Diagnostics) {
	var diags diag.Diagnostics
	hasID := !organizationID.IsNull() && !organizationID.IsUnknown() && organizationID.ValueString() != ""
	wantMe := me.ValueBool()
	switch {
	case hasID && wantMe:
		diags.AddError(
			"Conflicting organization selectors",
			"Set only one of `organization_id` or `me`, not both.",
		)
		return "", "", diags
	case wantMe:
		user, err := client.Users.GetCurrentUser(ctx)
		if err != nil {
			diags.AddError("Error resolving current user", formatAPIError(err))
			return "", "", diags
		}
		return user.OrganizationID, string(user.OrganizationID), diags
	case hasID:
		id := organizationID.ValueString()
		return models.OrganizationID(id), id, diags
	default:
		return "", "current", diags
	}
}
