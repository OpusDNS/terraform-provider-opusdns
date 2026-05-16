package provider

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// parkingAPIResponse mirrors common/schemas/parking.py:ParkingResponse.
type parkingAPIResponse struct {
	ParkingID        string    `json:"parking_id"`
	Domain           string    `json:"domain"`
	Enabled          bool      `json:"enabled"`
	ComplianceStatus *string   `json:"compliance_status"`
	ContentLanguage  *string   `json:"content_language"`
	Note             *string   `json:"note"`
	ContentURL       *string   `json:"content_url"`
	CreatedOn        time.Time `json:"created_on"`
	UpdatedOn        time.Time `json:"updated_on"`
}

// paginationMetadata mirrors common/schemas/api/response.py:PaginationMetadata.
type paginationMetadata struct {
	CurrentPage     int  `json:"current_page"`
	PageSize        int  `json:"page_size"`
	TotalPages      int  `json:"total_pages"`
	TotalItems      int  `json:"total_items"`
	HasNextPage     bool `json:"has_next_page"`
	HasPreviousPage bool `json:"has_previous_page"`
}

// parkingListResponse mirrors the paginated GET /v1/parking shape returned by
// the API's Pagination[ParkingResponse] wrapper:
// {"results": [...], "pagination": {...}}.
type parkingListResponse struct {
	Results    []parkingAPIResponse `json:"results"`
	Pagination paginationMetadata   `json:"pagination"`
}

// rawCreateParking wraps POST /v1/parking.
func rawCreateParking(ctx context.Context, c *opusdns.Client, body map[string]interface{}) (*parkingAPIResponse, error) {
	path := c.HTTPClient().BuildPath("parking")
	resp, err := c.HTTPClient().Post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var out parkingAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawGetParking wraps GET /v1/parking/{parking_reference}. The reference may
// be a parking_id (e.g. `parking_01j...`) or a domain name.
func rawGetParking(ctx context.Context, c *opusdns.Client, ref string) (*parkingAPIResponse, error) {
	path := c.HTTPClient().BuildPath("parking", ref)
	resp, err := c.HTTPClient().Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var out parkingAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawEnableParking wraps PATCH /v1/parking/{parking_reference}/enable.
func rawEnableParking(ctx context.Context, c *opusdns.Client, ref string) (*parkingAPIResponse, error) {
	path := c.HTTPClient().BuildPath("parking", ref, "enable")
	resp, err := c.HTTPClient().Patch(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var out parkingAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawDisableParking wraps PATCH /v1/parking/{parking_reference}/disable.
func rawDisableParking(ctx context.Context, c *opusdns.Client, ref string) (*parkingAPIResponse, error) {
	path := c.HTTPClient().BuildPath("parking", ref, "disable")
	resp, err := c.HTTPClient().Patch(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var out parkingAPIResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawDeleteParking wraps DELETE /v1/parking/{parking_reference}.
func rawDeleteParking(ctx context.Context, c *opusdns.Client, ref string) error {
	path := c.HTTPClient().BuildPath("parking", ref)
	resp, err := c.HTTPClient().Delete(ctx, path)
	if err != nil {
		return err
	}
	return c.HTTPClient().DecodeResponse(resp, nil)
}

// rawListParking wraps GET /v1/parking with optional filters. The API is
// paginated; this helper fetches up to `pageSize` per page and concatenates
// across pages until exhausted.
func rawListParking(ctx context.Context, c *opusdns.Client, filters map[string]string) ([]parkingAPIResponse, error) {
	pageSize := 100
	var all []parkingAPIResponse
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("page", fmt.Sprintf("%d", page))
		q.Set("page_size", fmt.Sprintf("%d", pageSize))
		for k, v := range filters {
			if v != "" {
				q.Set(k, v)
			}
		}
		path := c.HTTPClient().BuildPath("parking")
		resp, err := c.HTTPClient().Get(ctx, path, q)
		if err != nil {
			return nil, err
		}
		var out parkingListResponse
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

// applyParkingToModel writes ParkingResponse fields onto a ParkingResourceModel.
// Shared between Create / Read / Update paths.
func applyParkingToModel(data *ParkingResourceModel, p *parkingAPIResponse, _ *diag.Diagnostics) {
	data.ID = types.StringValue(p.ParkingID)
	data.ParkingID = types.StringValue(p.ParkingID)
	data.Domain = types.StringValue(p.Domain)
	data.Enabled = types.BoolValue(p.Enabled)
	data.ComplianceStatus = stringPtrToValue(p.ComplianceStatus)
	data.ContentLanguage = stringPtrToValue(p.ContentLanguage)
	data.Note = stringPtrToValue(p.Note)
	data.ContentURL = stringPtrToValue(p.ContentURL)
	data.CreatedOn = types.StringValue(p.CreatedOn.Format(time.RFC3339))
	data.UpdatedOn = types.StringValue(p.UpdatedOn.Format(time.RFC3339))
}
