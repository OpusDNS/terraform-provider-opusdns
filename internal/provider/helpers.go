package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// isNotFound returns true when an API error indicates a 404 Not Found response.
func isNotFound(err error) bool {
	return errors.Is(err, opusdns.ErrNotFound)
}

// addOptionalString sets body[key] = value only when v is a real, set string.
// Used to assemble JSON request bodies that omit unspecified optional fields.
func addOptionalString(body map[string]interface{}, key string, v types.String) {
	if v.IsNull() || v.IsUnknown() {
		return
	}
	body[key] = v.ValueString()
}

// optionalStringPtr returns a *string for a types.String, or nil when null/unknown.
// Useful for SDK request types whose optional fields are *string.
func optionalStringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// stringPtrToValue converts a *string (typical of SDK response types) into a
// types.String, mapping nil to types.StringNull().
func stringPtrToValue(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// rawCreateOrganization wraps POST /v1/organizations. The SDK as of v1.0.9 has
// no CreateOrganization helper, so we issue the call directly through the
// SDK's underlying HTTPClient — this preserves the bearer-token transport
// configured by the provider.
func rawCreateOrganization(ctx context.Context, c *opusdns.Client, body map[string]interface{}) (*models.Organization, error) {
	path := c.HTTPClient().BuildPath("organizations")
	resp, err := c.HTTPClient().Post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var org models.Organization
	if err := c.HTTPClient().DecodeResponse(resp, &org); err != nil {
		return nil, err
	}
	return &org, nil
}

// rawDeleteOrganization wraps DELETE /v1/organizations/{id}. The SDK has no
// DeleteOrganization helper, so we issue it via the underlying HTTPClient.
func rawDeleteOrganization(ctx context.Context, c *opusdns.Client, orgID models.OrganizationID) error {
	path := c.HTTPClient().BuildPath("organizations", string(orgID))
	resp, err := c.HTTPClient().Delete(ctx, path)
	if err != nil {
		return err
	}
	return c.HTTPClient().DecodeResponse(resp, nil)
}
