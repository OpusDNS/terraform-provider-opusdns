package provider

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/opusdns/opusdns-go-client/opusdns"
)

// contactAttributeSetAPIResponse mirrors ContactAttributeSetResponse from the
// OpenAPI spec (see /v1/contacts/attribute-sets endpoints).
type contactAttributeSetAPIResponse struct {
	ContactAttributeSetID string            `json:"contact_attribute_set_id"`
	OrganizationID        string            `json:"organization_id"`
	Label                 string            `json:"label"`
	TLD                   string            `json:"tld"`
	Attributes            map[string]string `json:"attributes"`
	LinkedContacts        int64             `json:"linked_contacts"`
	CreatedOn             time.Time         `json:"created_on"`
	UpdatedOn             time.Time         `json:"updated_on"`
}

// contactAttributeSetListResponse mirrors Pagination[ContactAttributeSetResponse].
type contactAttributeSetListResponse struct {
	Pagination paginationMetadata               `json:"pagination"`
	Results    []contactAttributeSetAPIResponse `json:"results"`
}

// rawCreateContactAttributeSet wraps POST /v1/contacts/attribute-sets.
func rawCreateContactAttributeSet(ctx context.Context, c *opusdns.Client, body map[string]interface{}) (*contactAttributeSetAPIResponse, error) {
	path := c.HTTPClient().BuildPath("contacts", "attribute-sets")
	resp, err := c.HTTPClient().Post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var out contactAttributeSetAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawGetContactAttributeSet wraps GET /v1/contacts/attribute-sets/{id}.
func rawGetContactAttributeSet(ctx context.Context, c *opusdns.Client, id string) (*contactAttributeSetAPIResponse, error) {
	path := c.HTTPClient().BuildPath("contacts", "attribute-sets", id)
	resp, err := c.HTTPClient().Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var out contactAttributeSetAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawUpdateContactAttributeSet wraps PATCH /v1/contacts/attribute-sets/{id}.
// Per the OpenAPI spec only `label` is updatable.
func rawUpdateContactAttributeSet(ctx context.Context, c *opusdns.Client, id string, body map[string]interface{}) (*contactAttributeSetAPIResponse, error) {
	path := c.HTTPClient().BuildPath("contacts", "attribute-sets", id)
	resp, err := c.HTTPClient().Patch(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var out contactAttributeSetAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawDeleteContactAttributeSet wraps DELETE /v1/contacts/attribute-sets/{id}.
func rawDeleteContactAttributeSet(ctx context.Context, c *opusdns.Client, id string) error {
	path := c.HTTPClient().BuildPath("contacts", "attribute-sets", id)
	resp, err := c.HTTPClient().Delete(ctx, path)
	if err != nil {
		return err
	}
	return c.HTTPClient().DecodeResponse(resp, nil)
}

// rawListContactAttributeSets wraps GET /v1/contacts/attribute-sets, walking
// pages until the API reports no further next page.
func rawListContactAttributeSets(ctx context.Context, c *opusdns.Client, tld, label string) ([]contactAttributeSetAPIResponse, error) {
	var all []contactAttributeSetAPIResponse
	page := 1
	for {
		query := url.Values{}
		query.Set("page", strconv.Itoa(page))
		query.Set("page_size", "100")
		if tld != "" {
			query.Set("tld", tld)
		}
		if label != "" {
			query.Set("label", label)
		}
		path := c.HTTPClient().BuildPath("contacts", "attribute-sets")
		resp, err := c.HTTPClient().Get(ctx, path, query)
		if err != nil {
			return nil, err
		}
		var pageResp contactAttributeSetListResponse
		if err := c.HTTPClient().DecodeResponse(resp, &pageResp); err != nil {
			return nil, err
		}
		all = append(all, pageResp.Results...)
		if !pageResp.Pagination.HasNextPage {
			break
		}
		page++
	}
	return all, nil
}

// contactAttributeLinkAPIResponse mirrors ContactAttributeLinkResponse.
type contactAttributeLinkAPIResponse struct {
	ContactAttributeLinkID string    `json:"contact_attribute_link_id"`
	ContactID              string    `json:"contact_id"`
	ContactAttributeSetID  string    `json:"contact_attribute_set_id"`
	TLD                    string    `json:"tld"`
	CreatedOn              time.Time `json:"created_on"`
}

// rawLinkContactToAttributeSet wraps
// PATCH /v1/contacts/{contact_id}/link/{contact_attribute_set_id}.
func rawLinkContactToAttributeSet(ctx context.Context, c *opusdns.Client, contactID, setID string) (*contactAttributeLinkAPIResponse, error) {
	path := c.HTTPClient().BuildPath("contacts", contactID, "link", setID)
	resp, err := c.HTTPClient().Patch(ctx, path, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	var out contactAttributeLinkAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawGetContact reads /v1/contacts/{id} and returns the linked-attribute-sets
// portion of the body. The provider already has a typed contacts service, but
// the SDK at v1.0.9 may not expose `attribute_sets` on its Contact response;
// we decode the relevant subset directly to keep this isolated.
func rawGetContactAttributeLinks(ctx context.Context, c *opusdns.Client, contactID string) ([]contactAttributeLinkDetail, error) {
	path := c.HTTPClient().BuildPath("contacts", contactID)
	resp, err := c.HTTPClient().Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		AttributeSets []contactAttributeLinkDetail `json:"attribute_sets"`
	}
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return out.AttributeSets, nil
}

// contactAttributeLinkDetail mirrors the embedded ContactAttributeLinkDetail
// returned within ContactResponse.attribute_sets.
type contactAttributeLinkDetail struct {
	ContactAttributeSetID string            `json:"contact_attribute_set_id"`
	Label                 string            `json:"label"`
	TLD                   string            `json:"tld"`
	Attributes            map[string]string `json:"attributes"`
}
