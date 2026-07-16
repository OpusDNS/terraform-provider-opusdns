package provider

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// domainWithStatusTags embeds the SDK Domain model and augments it with the
// `status_tags` array the API returns alongside `tags` when `include=tags` is
// requested. The SDK's models.Domain has no status_tags field, so we decode
// into this wrapper via the raw HTTP client.
//
// Embedding models.Domain means all of the standard domain fields decode
// exactly as the SDK would decode them; only status_tags is added.
type domainWithStatusTags struct {
	models.Domain
	StatusTags []models.StatusTagResponse `json:"status_tags,omitempty"`
}

// domainListWithStatusTagsResponse mirrors the paginated `{results,
// pagination}` envelope returned by GET /v1/domains, decoding each row into a
// domainWithStatusTags.
type domainListWithStatusTagsResponse struct {
	Results    []domainWithStatusTags `json:"results"`
	Pagination models.Pagination      `json:"pagination"`
}

// rawListDomainsWithStatusTags wraps GET /v1/domains, adding the status-tag
// filter parameters the SDK's ListDomainsOptions cannot express
// (`status_tags`, `status_tag_mode`) and decoding the `status_tags` array the
// SDK's models.Domain omits. It always requests `include=tags` so status_tags
// (and user tags) are populated. Auto-paginates.
//
// It reuses the SDK's own query-building for the shared filters by delegating
// to buildDomainListQuery, then appends the status-tag params. This is only
// used when the caller sets status-tag filters or needs status_tags in the
// response; the plain SDK path handles every other case (see datasource_domains).
func rawListDomainsWithStatusTags(
	ctx context.Context,
	c *opusdns.Client,
	opts *models.ListDomainsOptions,
	statusTags []string,
	statusTagMode string,
) ([]domainWithStatusTags, error) {
	pageSize := 100
	var all []domainWithStatusTags
	for page := 1; ; page++ {
		q := buildDomainListQuery(opts)
		q.Set("page", strconv.Itoa(page))
		q.Set("page_size", strconv.Itoa(pageSize))
		q.Set("include", string(models.DomainIncludeTags))
		for _, t := range statusTags {
			q.Add("status_tags", t)
		}
		if statusTagMode != "" {
			q.Set("status_tag_mode", statusTagMode)
		}

		path := c.HTTPClient().BuildPath("domains")
		resp, err := c.HTTPClient().Get(ctx, path, q)
		if err != nil {
			return nil, err
		}
		var out domainListWithStatusTagsResponse
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

// rawGetDomainWithStatusTags wraps GET /v1/domains/{ref}?include=tags,
// decoding the `status_tags` array the SDK's models.Domain omits.
func rawGetDomainWithStatusTags(ctx context.Context, c *opusdns.Client, domainRef string) (*domainWithStatusTags, error) {
	q := url.Values{}
	q.Add("include", string(models.DomainIncludeTags))
	path := c.HTTPClient().BuildPath("domains", url.PathEscape(domainRef))
	resp, err := c.HTTPClient().Get(ctx, path, q)
	if err != nil {
		return nil, err
	}
	var out domainWithStatusTags
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rawCreateDomain wraps POST /v1/domains for the registration path when the
// SDK's DomainCreateRequest cannot express the field(s) we need
// (`expected_price`, `claims_notice_acceptance_hash`). The body is assembled by
// the caller; the response decodes into the standard SDK Domain model.
func rawCreateDomain(ctx context.Context, c *opusdns.Client, body map[string]interface{}) (*models.Domain, error) {
	path := c.HTTPClient().BuildPath("domains")
	resp, err := c.HTTPClient().Post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	var out models.Domain
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// domainCreateRequestToMap serialises an SDK DomainCreateRequest into a generic
// JSON map so create-only fields the SDK type cannot express (expected_price,
// claims_notice_acceptance_hash) can be added before a raw POST. Using the
// SDK's own json tags via a round-trip keeps the body identical to what the SDK
// would send for the shared fields.
func domainCreateRequestToMap(req *models.DomainCreateRequest) (map[string]interface{}, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// buildDomainListQuery reproduces the SDK's ListDomainsPage query construction
// for the shared domain-list filters, so the raw status-tag wrapper does not
// regress any existing filter. Keep this in sync with the SDK's
// service_domains.go query building.
func buildDomainListQuery(opts *models.ListDomainsOptions) url.Values {
	q := url.Values{}
	if opts == nil {
		return q
	}
	if opts.SortBy != "" {
		q.Set("sort_by", string(opts.SortBy))
	}
	if opts.SortOrder != "" {
		q.Set("sort_order", string(opts.SortOrder))
	}
	for _, tagID := range opts.TagIDs {
		q.Add("tag_ids", string(tagID))
	}
	if opts.TagMode != "" {
		q.Set("tag_mode", string(opts.TagMode))
	}
	if opts.Search != "" {
		q.Set("search", opts.Search)
	}
	if opts.Name != "" {
		q.Set("name", opts.Name)
	}
	if opts.TLD != "" {
		q.Set("tld", opts.TLD)
	}
	if opts.SLD != "" {
		q.Set("sld", opts.SLD)
	}
	if opts.TransferLock != nil {
		q.Set("transfer_lock", strconv.FormatBool(*opts.TransferLock))
	}
	if opts.IsPremium != nil {
		q.Set("is_premium", strconv.FormatBool(*opts.IsPremium))
	}
	if opts.RenewalMode != nil {
		q.Set("renewal_mode", string(*opts.RenewalMode))
	}
	if opts.CreatedAfter != nil {
		q.Set("created_after", opts.CreatedAfter.Format(time.RFC3339))
	}
	if opts.CreatedBefore != nil {
		q.Set("created_before", opts.CreatedBefore.Format(time.RFC3339))
	}
	if opts.UpdatedAfter != nil {
		q.Set("updated_after", opts.UpdatedAfter.Format(time.RFC3339))
	}
	if opts.UpdatedBefore != nil {
		q.Set("updated_before", opts.UpdatedBefore.Format(time.RFC3339))
	}
	if opts.ExpiresAfter != nil {
		q.Set("expires_after", opts.ExpiresAfter.Format(time.RFC3339))
	}
	if opts.ExpiresBefore != nil {
		q.Set("expires_before", opts.ExpiresBefore.Format(time.RFC3339))
	}
	if opts.ExpiresIn30Days != nil {
		q.Set("expires_in_30_days", strconv.FormatBool(*opts.ExpiresIn30Days))
	}
	if opts.ExpiresIn60Days != nil {
		q.Set("expires_in_60_days", strconv.FormatBool(*opts.ExpiresIn60Days))
	}
	if opts.ExpiresIn90Days != nil {
		q.Set("expires_in_90_days", strconv.FormatBool(*opts.ExpiresIn90Days))
	}
	if opts.RegisteredAfter != nil {
		q.Set("registered_after", opts.RegisteredAfter.Format(time.RFC3339))
	}
	if opts.RegisteredBefore != nil {
		q.Set("registered_before", opts.RegisteredBefore.Format(time.RFC3339))
	}
	for _, status := range opts.RegistryStatuses {
		q.Add("registry_statuses", status)
	}
	if opts.Status != "" {
		q.Set("status", string(opts.Status))
	}
	return q
}
