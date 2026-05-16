package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// listToStringSlice extracts a []string from a types.List of StringType.
// Returns an empty slice (and no diagnostics) when the list is null or
// unknown so callers can pass the result straight to a JSON body.
func listToStringSlice(ctx context.Context, l types.List) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if l.IsNull() || l.IsUnknown() {
		return []string{}, diags
	}
	out := make([]string, 0, len(l.Elements()))
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out, diags
}

// rawCreateHost wraps POST /v1/hosts.
// The SDK at v1.0.9 has no typed Hosts service, so we issue the call directly
// through the underlying HTTPClient, which preserves the provider's
// bearer-token transport.
func rawCreateHost(ctx context.Context, c *opusdns.Client, body map[string]interface{}) (*hostAPIResponse, error) {
	path := c.HTTPClient().BuildPath("hosts")
	resp, err := c.HTTPClient().Post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var out hostAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawGetHost wraps GET /v1/hosts/{host_reference}. The reference may be a
// host_id (e.g. `host_01j...`) or a hostname.
func rawGetHost(ctx context.Context, c *opusdns.Client, hostRef string) (*hostAPIResponse, error) {
	path := c.HTTPClient().BuildPath("hosts", hostRef)
	resp, err := c.HTTPClient().Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var out hostAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawUpdateHost wraps PUT /v1/hosts/{host_reference}.
func rawUpdateHost(ctx context.Context, c *opusdns.Client, hostRef string, body map[string]interface{}) (*hostAPIResponse, error) {
	path := c.HTTPClient().BuildPath("hosts", hostRef)
	resp, err := c.HTTPClient().Put(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var out hostAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawDeleteHost wraps DELETE /v1/hosts/{host_reference}.
func rawDeleteHost(ctx context.Context, c *opusdns.Client, hostRef string) error {
	path := c.HTTPClient().BuildPath("hosts", hostRef)
	resp, err := c.HTTPClient().Delete(ctx, path)
	if err != nil {
		return err
	}
	return c.HTTPClient().DecodeResponse(resp, nil)
}
