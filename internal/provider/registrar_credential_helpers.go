package provider

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/opusdns/opusdns-go-client/opusdns"
)

// registrarCredentialAPIResponse mirrors
// common/schemas/registrar_credential.py:RegistrarCredential. Note: the
// `credentials` payload is NOT returned by the API (only the
// RegistrarCredentialWithSecret variant carries it, and no endpoint
// returns that variant). Reads therefore yield metadata only.
type registrarCredentialAPIResponse struct {
	RegistrarCredentialID string    `json:"registrar_credential_id"`
	OrganizationID        string    `json:"organization_id"`
	Name                  string    `json:"name"`
	Registrar             string    `json:"registrar"`
	CreatedOn             time.Time `json:"created_on"`
	UpdatedOn             time.Time `json:"updated_on"`
}

// registrarCredentialListResponse mirrors Pagination[RegistrarCredential]
// from common/schemas/api/response.py — the API returns `{results,
// pagination}` (NOT `{items, total, page, ...}`).
type registrarCredentialListResponse struct {
	Results    []registrarCredentialAPIResponse `json:"results"`
	Pagination paginationMetadata               `json:"pagination"`
}

// paginationMetadata is defined in parking_helpers.go (same package).

// rawCreateRegistrarCredential wraps POST /v1/connect/registrars.
func rawCreateRegistrarCredential(ctx context.Context, c *opusdns.Client, body map[string]interface{}) (*registrarCredentialAPIResponse, error) {
	path := c.HTTPClient().BuildPath("connect", "registrars")
	resp, err := c.HTTPClient().Post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var out registrarCredentialAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawGetRegistrarCredential wraps GET /v1/connect/registrars/{id}.
func rawGetRegistrarCredential(ctx context.Context, c *opusdns.Client, id string) (*registrarCredentialAPIResponse, error) {
	path := c.HTTPClient().BuildPath("connect", "registrars", id)
	resp, err := c.HTTPClient().Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var out registrarCredentialAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawUpdateRegistrarCredential wraps PUT /v1/connect/registrars/{id}.
// The API only accepts `credentials` on update; the human-readable name
// and registrar are immutable.
func rawUpdateRegistrarCredential(ctx context.Context, c *opusdns.Client, id string, body map[string]interface{}) (*registrarCredentialAPIResponse, error) {
	path := c.HTTPClient().BuildPath("connect", "registrars", id)
	resp, err := c.HTTPClient().Put(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var out registrarCredentialAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawDeleteRegistrarCredential wraps DELETE /v1/connect/registrars/{id}.
func rawDeleteRegistrarCredential(ctx context.Context, c *opusdns.Client, id string) error {
	path := c.HTTPClient().BuildPath("connect", "registrars", id)
	resp, err := c.HTTPClient().Delete(ctx, path)
	if err != nil {
		return err
	}
	return c.HTTPClient().DecodeResponse(resp, nil)
}

// rawListRegistrarCredentials wraps GET /v1/connect/registrars with optional
// registrar filter. Auto-paginates.
func rawListRegistrarCredentials(ctx context.Context, c *opusdns.Client, registrar string) ([]registrarCredentialAPIResponse, error) {
	pageSize := 100
	var all []registrarCredentialAPIResponse
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("page_size", fmt.Sprintf("%d", pageSize))
		if registrar != "" {
			q.Set("registrar", registrar)
		}
		path := c.HTTPClient().BuildPath("connect", "registrars")
		resp, err := c.HTTPClient().Get(ctx, path, q)
		if err != nil {
			return nil, err
		}
		var out registrarCredentialListResponse
		if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
			return nil, err
		}
		all = append(all, out.Results...)
		if !out.Pagination.HasNextPage {
			break
		}
	}
	return all, nil
}
