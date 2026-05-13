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
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure UserResource satisfies the resource.Resource interface.
var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

// UserResource manages users via /v1/users. The user is provisioned under the
// authenticated caller's organization.
type UserResource struct {
	client *opusdns.Client
}

// UserResourceModel mirrors models.UserCreateRequest + UserUpdateRequest plus
// computed fields.
type UserResourceModel struct {
	ID             types.String `tfsdk:"id"`
	UserID         types.String `tfsdk:"user_id"`
	Username       types.String `tfsdk:"username"`
	FirstName      types.String `tfsdk:"first_name"`
	LastName       types.String `tfsdk:"last_name"`
	Email          types.String `tfsdk:"email"`
	Phone          types.String `tfsdk:"phone"`
	Locale         types.String `tfsdk:"locale"`
	Status         types.String `tfsdk:"status"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedOn      types.String `tfsdk:"created_on"`
	UpdatedOn      types.String `tfsdk:"updated_on"`
}

// NewUserResource returns a new UserResource.
func NewUserResource() resource.Resource {
	return &UserResource{}
}

func (r *UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	useStateForUnknown := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a user in OpusDNS via `/v1/users`. Users are provisioned under the authenticated caller's organization. " +
			"`username` and `email` are immutable; changing either replaces the user.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mirror of `user_id`.",
				PlanModifiers:       useStateForUnknown,
			},
			"user_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the user, e.g. `user_...`.",
				PlanModifiers:       useStateForUnknown,
			},
			"username": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Unique username for the user. Immutable.",
				PlanModifiers:       requiresReplace,
			},
			"email": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Email address. Immutable.",
				PlanModifiers:       requiresReplace,
			},
			"first_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "First name.",
			},
			"last_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Last name.",
			},
			"phone": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Phone number in E.164 format.",
				PlanModifiers:       useStateForUnknown,
			},
			"locale": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "User locale (e.g. `en-US`).",
				PlanModifiers:       useStateForUnknown,
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "User status (`active`, `inactive`, `pending`).",
				PlanModifiers:       useStateForUnknown,
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization the user belongs to.",
				PlanModifiers:       useStateForUnknown,
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp the user was created.",
				PlanModifiers:       useStateForUnknown,
			},
			"updated_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp the user was last updated.",
				PlanModifiers:       useStateForUnknown,
			},
		},
	}
}

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.UserCreateRequest{
		Username:  data.Username.ValueString(),
		FirstName: data.FirstName.ValueString(),
		LastName:  data.LastName.ValueString(),
		Email:     data.Email.ValueString(),
		Locale:    data.Locale.ValueString(),
	}
	createReq.Phone = optionalStringPtr(data.Phone)

	user, err := r.client.Users.CreateUser(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating user", formatAPIError(err))
		return
	}

	populateUserModel(&data, user)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.Users.GetUser(ctx, models.UserID(data.UserID.ValueString()))
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading user", formatAPIError(err))
		return
	}

	populateUserModel(&data, user)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan UserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &models.UserUpdateRequest{
		FirstName: optionalStringPtr(plan.FirstName),
		LastName:  optionalStringPtr(plan.LastName),
		Phone:     optionalStringPtr(plan.Phone),
		Locale:    optionalStringPtr(plan.Locale),
	}

	user, err := r.client.Users.UpdateUser(ctx, models.UserID(plan.UserID.ValueString()), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating user", formatAPIError(err))
		return
	}

	populateUserModel(&plan, user)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Users.DeleteUser(ctx, models.UserID(data.UserID.ValueString())); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting user", formatAPIError(err))
		}
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// populateUserModel maps an API User onto the TF model.
func populateUserModel(data *UserResourceModel, u *models.User) {
	data.ID = types.StringValue(string(u.UserID))
	data.UserID = types.StringValue(string(u.UserID))
	data.Username = types.StringValue(u.Username)
	data.FirstName = types.StringValue(u.FirstName)
	data.LastName = types.StringValue(u.LastName)
	data.Email = types.StringValue(u.Email)
	data.Locale = types.StringValue(u.Locale)
	data.Status = types.StringValue(string(u.Status))
	data.OrganizationID = types.StringValue(string(u.OrganizationID))

	data.Phone = stringPtrToValue(u.Phone)

	if u.CreatedOn != nil {
		data.CreatedOn = types.StringValue(u.CreatedOn.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		data.CreatedOn = types.StringNull()
	}
	if u.UpdatedOn != nil {
		data.UpdatedOn = types.StringValue(u.UpdatedOn.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		data.UpdatedOn = types.StringNull()
	}
}
