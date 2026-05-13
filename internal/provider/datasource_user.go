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

// Ensure UserDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &UserDataSource{}

// UserDataSource fetches a single user. Set `me = true` to fetch the
// authenticated caller's user record (`GET /v1/users/me`).
type UserDataSource struct {
	client *opusdns.Client
}

// UserDataSourceModel matches UserResourceModel with `me` and an optional `user_id`.
type UserDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	UserID         types.String `tfsdk:"user_id"`
	Me             types.Bool   `tfsdk:"me"`
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

// NewUserDataSource returns a new UserDataSource.
func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

func (d *UserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single user. Set `me = true` to look up the authenticated caller via `GET /v1/users/me`, " +
			"otherwise provide `user_id` to look up `GET /v1/users/{user_id}`.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Mirror of `user_id`."},
			"user_id":         schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "User id to look up. Required unless `me = true`."},
			"me":              schema.BoolAttribute{Optional: true, MarkdownDescription: "When true, resolve via `/v1/users/me`."},
			"username":        schema.StringAttribute{Computed: true},
			"first_name":      schema.StringAttribute{Computed: true},
			"last_name":       schema.StringAttribute{Computed: true},
			"email":           schema.StringAttribute{Computed: true},
			"phone":           schema.StringAttribute{Computed: true},
			"locale":          schema.StringAttribute{Computed: true},
			"status":          schema.StringAttribute{Computed: true},
			"organization_id": schema.StringAttribute{Computed: true},
			"created_on":      schema.StringAttribute{Computed: true},
			"updated_on":      schema.StringAttribute{Computed: true},
		},
	}
}

func (d *UserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var user *models.User
	var err error
	switch {
	case data.Me.ValueBool():
		user, err = d.client.Users.GetCurrentUser(ctx)
	case !data.UserID.IsNull() && !data.UserID.IsUnknown() && data.UserID.ValueString() != "":
		user, err = d.client.Users.GetUser(ctx, models.UserID(data.UserID.ValueString()))
	default:
		resp.Diagnostics.AddError(
			"Missing user selector",
			"Either set `user_id` or `me = true`.",
		)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading user", formatAPIError(err))
		return
	}

	populateUserDataModel(&data, user)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func populateUserDataModel(data *UserDataSourceModel, u *models.User) {
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
