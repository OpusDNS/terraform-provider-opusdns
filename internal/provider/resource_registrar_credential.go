package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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

// Ensure RegistrarCredentialResource satisfies the resource.Resource interface.
var _ resource.Resource = &RegistrarCredentialResource{}
var _ resource.ResourceWithImportState = &RegistrarCredentialResource{}

// RegistrarCredentialResource implements `opusdns_registrar_credential`
// backed by `/v1/connect/registrars`. A registrar credential stores the
// authentication material (API keys, tokens, etc.) the OpusDNS connect
// service uses to act on the organization's behalf at a third-party
// registrar.
//
// The SDK at v1.0.9 does not expose a typed Connect/Registrars service,
// so this resource is wired directly to the SDK's low-level HTTPClient.
//
// Important: the `credentials` map is write-only at the API level — the
// GET response never includes it. The provider treats `credentials` as
// sensitive write-only state: it is persisted in Terraform state as
// supplied (and required for plan stability), but the provider cannot
// detect drift against the registrar after the initial write. To rotate
// or repair credentials, apply a new plan with the desired values.
//
// `name` and `registrar` are immutable at the API level (the PUT body
// accepts only `credentials`); changes to either force replacement.
type RegistrarCredentialResource struct {
	client *opusdns.Client
}

// RegistrarCredentialResourceModel is the state shape for the resource.
type RegistrarCredentialResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	RegistrarCredentialID types.String `tfsdk:"registrar_credential_id"`
	OrganizationID        types.String `tfsdk:"organization_id"`
	Name                  types.String `tfsdk:"name"`
	Registrar             types.String `tfsdk:"registrar"`
	Credentials           types.Map    `tfsdk:"credentials"`
	CreatedOn             types.String `tfsdk:"created_on"`
	UpdatedOn             types.String `tfsdk:"updated_on"`
}

// NewRegistrarCredentialResource returns a new RegistrarCredentialResource.
func NewRegistrarCredentialResource() resource.Resource {
	return &RegistrarCredentialResource{}
}

func (r *RegistrarCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registrar_credential"
}

func (r *RegistrarCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	useStateForUnknown := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a third-party registrar credential in OpusDNS via `/v1/connect/registrars`. " +
			"A credential stores the authentication material (API keys, tokens, etc.) the OpusDNS connect " +
			"service uses to act on the organization's behalf at the given registrar.\n\n" +
			"**Credentials handling**: the `credentials` map is sensitive and write-only at the API level — " +
			"the GET response does not include it. The provider persists the supplied values in state for plan " +
			"stability but cannot detect drift against the registrar. To rotate or repair, apply a new plan with " +
			"the desired values.\n\n" +
			"**Immutability**: `name` and `registrar` cannot be changed in place; modifying either forces replacement.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The registrar credential id (mirrors `registrar_credential_id`).",
				PlanModifiers:       useStateForUnknown,
			},
			"registrar_credential_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the registrar credential (e.g. `registrar_credential_01j...`).",
				PlanModifiers:       useStateForUnknown,
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization that owns this credential.",
				PlanModifiers:       useStateForUnknown,
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name for the credential. 1-255 characters. Immutable; changes force replacement.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"registrar": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The third-party registrar this credential targets. One of `INTERNETX`, `MONIKER`, " +
					"`DOMAIN_BESTELLSYSTEM`, `CENTRALNIC`, `OPUSDNS`, `ENOM`. Immutable; changes force replacement.",
				Validators: []validator.String{
					stringvalidator.OneOf("INTERNETX", "MONIKER", "DOMAIN_BESTELLSYSTEM", "CENTRALNIC", "OPUSDNS", "ENOM"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credentials": schema.MapAttribute{
				Required:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
				MarkdownDescription: "Registrar-specific credential material as a map of strings (e.g. `api_key`, `username`, `password`, `endpoint`). The required keys are determined by the chosen `registrar`. The API does not return this field on read, so the provider cannot detect drift against the registrar — apply a new plan to rotate values.",
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the credential was created.",
				PlanModifiers:       useStateForUnknown,
			},
			"updated_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the credential was last updated.",
			},
		},
	}
}

func (r *RegistrarCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// credentialsMapToAPI converts the Terraform map(string) into the
// `dict[str, Any]` body expected by the API.
func credentialsMapToAPI(ctx context.Context, m types.Map) (map[string]interface{}, diag.Diagnostics) {
	out := map[string]interface{}{}
	if m.IsNull() || m.IsUnknown() {
		return out, nil
	}
	raw := map[string]string{}
	d := m.ElementsAs(ctx, &raw, false)
	if d.HasError() {
		return nil, d
	}
	for k, v := range raw {
		out[k] = v
	}
	return out, d
}

func (r *RegistrarCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RegistrarCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	creds, diags := credentialsMapToAPI(ctx, data.Credentials)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]interface{}{
		"name":        data.Name.ValueString(),
		"registrar":   data.Registrar.ValueString(),
		"credentials": creds,
	}

	cred, err := rawCreateRegistrarCredential(ctx, r.client, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating registrar credential", formatAPIError(err))
		return
	}

	applyRegistrarCredentialToModel(&data, cred)
	// Preserve the plan's credentials map in state (server does not return it).
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RegistrarCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RegistrarCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.RegistrarCredentialID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError(
			"Invalid registrar credential state",
			"The opusdns_registrar_credential resource has an empty `registrar_credential_id` in state, which prevents reading it from the API. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	cred, err := rawGetRegistrarCredential(ctx, r.client, id)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading registrar credential", formatAPIError(err))
		return
	}

	// Refresh server-owned fields only; preserve the existing credentials map.
	applyRegistrarCredentialToModel(&data, cred)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RegistrarCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state RegistrarCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.RegistrarCredentialID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError(
			"Invalid registrar credential state",
			"The opusdns_registrar_credential resource has an empty `registrar_credential_id` in state, which prevents updating it. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	var cred *registrarCredentialAPIResponse
	if !plan.Credentials.Equal(state.Credentials) {
		creds, diags := credentialsMapToAPI(ctx, plan.Credentials)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updated, err := rawUpdateRegistrarCredential(ctx, r.client, id, map[string]interface{}{
			"credentials": creds,
		})
		if err != nil {
			resp.Diagnostics.AddError("Error updating registrar credential", formatAPIError(err))
			return
		}
		cred = updated
	} else {
		current, err := rawGetRegistrarCredential(ctx, r.client, id)
		if err != nil {
			resp.Diagnostics.AddError("Error reading registrar credential", formatAPIError(err))
			return
		}
		cred = current
	}

	applyRegistrarCredentialToModel(&plan, cred)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RegistrarCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RegistrarCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.RegistrarCredentialID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError(
			"Invalid registrar credential state",
			"The opusdns_registrar_credential resource has an empty `registrar_credential_id` in state, which prevents deletion via the API. "+
				"Remove the resource from state with `terraform state rm` and, if the credential still exists at OpusDNS, delete it manually or re-import then destroy.",
		)
		return
	}

	if err := rawDeleteRegistrarCredential(ctx, r.client, id); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting registrar credential", formatAPIError(err))
		}
	}
}

func (r *RegistrarCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by registrar_credential_id only; credentials cannot be recovered
	// from the API and must be re-supplied via configuration after import.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("registrar_credential_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// applyRegistrarCredentialToModel copies server-owned fields from an API
// response onto the model. The `credentials` map is intentionally not
// touched — its value is governed by plan/state, not by the API.
func applyRegistrarCredentialToModel(data *RegistrarCredentialResourceModel, cred *registrarCredentialAPIResponse) {
	data.ID = types.StringValue(cred.RegistrarCredentialID)
	data.RegistrarCredentialID = types.StringValue(cred.RegistrarCredentialID)
	data.OrganizationID = types.StringValue(cred.OrganizationID)
	data.Name = types.StringValue(cred.Name)
	data.Registrar = types.StringValue(cred.Registrar)
	data.CreatedOn = types.StringValue(cred.CreatedOn.Format(time.RFC3339))
	data.UpdatedOn = types.StringValue(cred.UpdatedOn.Format(time.RFC3339))
}
