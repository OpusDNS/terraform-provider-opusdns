package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

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

// trimTrailingDot removes a single trailing `.` from a hostname. The OpusDNS
// API returns hostnames in fully-qualified form with a trailing dot (e.g.
// `example.com.`), but Terraform users supply, and naturally compare, the
// dot-less form. Strip on the way into state so plan-vs-state diffs and
// post-apply consistency checks align.
func trimTrailingDot(s string) string {
	return strings.TrimSuffix(s, ".")
}

// timePtrToValue converts a *time.Time (typical of SDK response types) into a
// types.String containing an RFC3339 representation, mapping nil to
// types.StringNull().
func timePtrToValue(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}

// formatAPIError returns a multi-line string describing err with as much
// detail as the SDK exposes. For *opusdns.APIError values it surfaces the
// status code, error code, message, request id, the raw response body, and
// any structured `details` map. For other errors it returns err.Error().
func formatAPIError(err error) string {
	var apiErr *opusdns.APIError
	if !errors.As(err, &apiErr) {
		return err.Error()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s", apiErr.Error())
	if apiErr.RequestID != "" {
		fmt.Fprintf(&b, "\nrequest_id: %s", apiErr.RequestID)
	}
	if len(apiErr.Details) > 0 {
		if data, mErr := json.MarshalIndent(apiErr.Details, "", "  "); mErr == nil {
			fmt.Fprintf(&b, "\ndetails:\n%s", data)
		} else {
			fmt.Fprintf(&b, "\ndetails: %+v", apiErr.Details)
		}
	}
	if apiErr.RawBody != "" {
		fmt.Fprintf(&b, "\nresponse body:\n%s", strings.TrimSpace(apiErr.RawBody))
	}
	return b.String()
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

// rawListEmailForwardsByZone wraps GET /v1/dns/{zone}/email-forwards.
//
// The SDK as of v1.0.9 declares this endpoint as returning a bare
// []models.EmailForward, but the API actually returns an EmailForwardZone
// wrapper object ({zone_id, zone_name, email_forwards: [...]}), which makes
// the SDK helper fail to decode. The sibling DomainForwards SDK service
// already handles this with a wrapper-first / list-fallback decode; this
// helper does the same for email forwards until the SDK is fixed upstream.
func rawListEmailForwardsByZone(ctx context.Context, c *opusdns.Client, zoneName string) ([]models.EmailForward, error) {
	path := c.HTTPClient().BuildPath("dns", url.PathEscape(zoneName), "email-forwards")
	resp, err := c.HTTPClient().Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	// Try the wrapper shape the API actually returns first; fall back to a
	// bare list for forwards-compatibility if the API ever returns one.
	var zone models.EmailForwardZone
	if decErr := c.HTTPClient().DecodeResponse(resp, &zone); decErr == nil {
		return zone.EmailForwards, nil
	}
	var list []models.EmailForward
	if decErr := c.HTTPClient().DecodeResponse(resp, &list); decErr != nil {
		return nil, decErr
	}
	return list, nil
}
