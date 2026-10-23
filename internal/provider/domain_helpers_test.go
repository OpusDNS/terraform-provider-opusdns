package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

func TestValidateDomainConfig_RegistrationRequiresContacts(t *testing.T) {
	data := DomainResourceModel{
		Transfer: types.BoolValue(false),
		Contacts: types.MapNull(contactsMapElemType),
	}
	diags := validateDomainConfig(data)
	if !diags.HasError() {
		t.Fatal("expected an error when registering without contacts")
	}
}

func TestValidateDomainConfig_RegistrationWithContactsOK(t *testing.T) {
	registrant, _ := types.ListValue(types.StringType, []attr.Value{types.StringValue("contact_1")})
	contacts, _ := types.MapValue(contactsMapElemType, map[string]attr.Value{
		"registrant": registrant,
	})
	data := DomainResourceModel{
		Transfer: types.BoolValue(false),
		Contacts: contacts,
	}
	diags := validateDomainConfig(data)
	if diags.HasError() {
		t.Fatalf("expected registration with contacts to be valid, got: %v", diags)
	}
}

func TestValidateDomainConfig_TransferWithoutContactsOK(t *testing.T) {
	data := DomainResourceModel{
		Transfer:   types.BoolValue(true),
		AuthCode:   types.StringValue("abc123"),
		CreateZone: types.BoolValue(false),
		PeriodUnit: types.StringValue("y"),
		Contacts:   types.MapNull(contactsMapElemType),
	}
	diags := validateDomainConfig(data)
	if diags.HasError() {
		t.Fatalf("expected transfer without contacts to be valid, got: %v", diags)
	}
}

func TestStatusTagTypesToStrings(t *testing.T) {
	tags := []models.StatusTagResponse{
		{TagType: models.StatusTagTypeVerificationRequired},
	}
	got := statusTagTypesToStrings(tags)
	if len(got) != 1 || got[0] != string(models.StatusTagTypeVerificationRequired) {
		t.Errorf("got %v, want [%s]", got, models.StatusTagTypeVerificationRequired)
	}
}

func TestFormatExpectedPrice(t *testing.T) {
	cases := map[float64]string{
		1250:    "1250.00",
		12.5:    "12.50",
		12.05:   "12.05",
		0.99:    "0.99",
		12.345:  "12.345", // not rounded: the API rejects it instead of confirming another price
		1234.56: "1234.56",
	}
	for in, want := range cases {
		if got := formatExpectedPrice(in); got != want {
			t.Errorf("formatExpectedPrice(%v) = %q, want %q", in, got, want)
		}
	}
}

// --- httptest-driven tests of the typed client calls ---

type recordedRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   string
}

// newRecordingClient returns a client talking to an httptest server that
// records every request and answers it through respond.
func newRecordingClient(t *testing.T, respond func(r recordedRequest) (int, string)) (*opusdns.Client, func() []recordedRequest) {
	t.Helper()
	var mu sync.Mutex
	var reqs []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec := recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: string(body)}
		mu.Lock()
		reqs = append(reqs, rec)
		mu.Unlock()
		status, payload := respond(rec)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)
	c, err := opusdns.NewClient(opusdns.WithAPIKey("opk_test"), opusdns.WithAPIEndpoint(srv.URL), opusdns.WithMaxRetries(0))
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	return c, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), reqs...)
	}
}

// objectWithOverrides builds a raw object value for the given schema type with
// every attribute null except the overrides.
func objectWithOverrides(t *testing.T, typ tftypes.Type, overrides map[string]tftypes.Value) tftypes.Value {
	t.Helper()
	objType, ok := typ.(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is %T, want tftypes.Object", typ)
	}
	vals := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, at := range objType.AttributeTypes {
		vals[name] = tftypes.NewValue(at, nil)
	}
	for name, v := range overrides {
		if _, ok := vals[name]; !ok {
			t.Fatalf("unknown attribute %q", name)
		}
		vals[name] = v
	}
	return tftypes.NewValue(objType, vals)
}

func tfStringList(vals ...string) tftypes.Value {
	out := make([]tftypes.Value, len(vals))
	for i, v := range vals {
		out[i] = tftypes.NewValue(tftypes.String, v)
	}
	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, out)
}

func readDataSourceWithOverrides(t *testing.T, ds datasource.DataSource, overrides map[string]tftypes.Value) datasource.ReadResponse {
	t.Helper()
	ctx := context.Background()
	var sresp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &sresp)
	typ := sresp.Schema.Type().TerraformType(ctx)
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: tftypes.NewValue(typ, nil)}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{
		Schema: sresp.Schema,
		Raw:    objectWithOverrides(t, typ, overrides),
	}}, &resp)
	return resp
}

const domainJSON = `{
  "domain_id": "domain_01h45ytscbebyvny4gc8cr8ma2",
  "name": "example.com",
  "sld": "example",
  "tld": "com",
  "roid": "123-COM",
  "owner_id": "organization_01h45ytscbebyvny4gc8cr8ma2",
  "registry_account_id": "registry_account_01h45ytscbebyvny4gc8cr8ma2",
  "renewal_mode": "renew",
  "registry_statuses": ["clientTransferProhibited"],
  "tags": [],
  "status_tags": [{"tag_type": "VERIFICATION_REQUIRED", "label": "Verification required", "color": "color-1"}]
}`

func TestDomainsDataSource_StatusTagFilterQuery(t *testing.T) {
	client, requests := newRecordingClient(t, func(r recordedRequest) (int, string) {
		page, _ := strconv.Atoi(r.Query.Get("page"))
		return 200, fmt.Sprintf(`{"results": [%s], "pagination": {"current_page": %d, "total_pages": 2, "has_next_page": %t, "page_size": 100}}`,
			domainJSON, page, page == 1)
	})

	resp := readDataSourceWithOverrides(t, &DomainsDataSource{client: client}, map[string]tftypes.Value{
		"tld":             tftypes.NewValue(tftypes.String, "com"),
		"status_tags":     tfStringList("VERIFICATION_REQUIRED", "CREATE_REQUESTED"),
		"status_tag_mode": tftypes.NewValue(tftypes.String, "match_all"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	reqs := requests()
	if len(reqs) != 2 {
		t.Fatalf("got %d requests, want 2 (one per page)", len(reqs))
	}
	for i, r := range reqs {
		if r.Method != http.MethodGet || r.Path != "/v1/domains" {
			t.Errorf("request %d = %s %s, want GET /v1/domains", i, r.Method, r.Path)
		}
		want := url.Values{
			"page":            {strconv.Itoa(i + 1)},
			"page_size":       {"100"},
			"tld":             {"com"},
			"include":         {"tags"},
			"status_tags":     {"VERIFICATION_REQUIRED", "CREATE_REQUESTED"},
			"status_tag_mode": {"match_all"},
		}
		if got, wantEnc := r.Query.Encode(), want.Encode(); got != wantEnc {
			t.Errorf("request %d query = %s, want %s", i, got, wantEnc)
		}
	}

	var state DomainsDataSourceModel
	resp.Diagnostics.Append(resp.State.Get(context.Background(), &state)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("reading state: %v", resp.Diagnostics)
	}
	if n := len(state.Domains.Elements()); n != 2 {
		t.Fatalf("got %d domains, want 2 (one per page)", n)
	}
	item := state.Domains.Elements()[0].(types.Object).Attributes()
	if got := item["status_tags"].String(); got != `["VERIFICATION_REQUIRED"]` {
		t.Errorf("status_tags = %s, want [\"VERIFICATION_REQUIRED\"]", got)
	}
}

func TestDomainsDataSource_PlainQueryHasNoStatusTagParams(t *testing.T) {
	client, requests := newRecordingClient(t, func(recordedRequest) (int, string) {
		return 200, `{"results": [` + domainJSON + `], "pagination": {"current_page": 1, "total_pages": 1, "has_next_page": false, "page_size": 100}}`
	})
	// status_tag_mode alone does not request status tags, as before.
	resp := readDataSourceWithOverrides(t, &DomainsDataSource{client: client}, map[string]tftypes.Value{
		"search":          tftypes.NewValue(tftypes.String, "exam"),
		"status_tag_mode": tftypes.NewValue(tftypes.String, "match_all"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("got %d requests, want 1", len(reqs))
	}
	want := url.Values{"page": {"1"}, "page_size": {"100"}, "search": {"exam"}}
	if got, wantEnc := reqs[0].Query.Encode(), want.Encode(); got != wantEnc {
		t.Errorf("query = %s, want %s", got, wantEnc)
	}
	var state DomainsDataSourceModel
	resp.Diagnostics.Append(resp.State.Get(context.Background(), &state)...)
	item := state.Domains.Elements()[0].(types.Object).Attributes()
	if n := len(item["status_tags"].(types.List).Elements()); n != 0 {
		t.Errorf("status_tags has %d elements, want 0 without include_tags", n)
	}
}

func TestDomainsDataSource_IncludeTagsOnly(t *testing.T) {
	client, requests := newRecordingClient(t, func(recordedRequest) (int, string) {
		return 200, `{"results": [` + domainJSON + `], "pagination": {"current_page": 1, "total_pages": 1, "has_next_page": false, "page_size": 100}}`
	})
	resp := readDataSourceWithOverrides(t, &DomainsDataSource{client: client}, map[string]tftypes.Value{
		"include_tags": tftypes.NewValue(tftypes.Bool, true),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	want := url.Values{"page": {"1"}, "page_size": {"100"}, "include": {"tags"}}
	if got, wantEnc := requests()[0].Query.Encode(), want.Encode(); got != wantEnc {
		t.Errorf("query = %s, want %s", got, wantEnc)
	}
}

func TestDomainDataSource_IncludeTagsReturnsStatusTags(t *testing.T) {
	for _, include := range []bool{true, false} {
		client, requests := newRecordingClient(t, func(recordedRequest) (int, string) { return 200, domainJSON })
		resp := readDataSourceWithOverrides(t, &DomainDataSource{client: client}, map[string]tftypes.Value{
			"domain_ref":   tftypes.NewValue(tftypes.String, "example.com"),
			"include_tags": tftypes.NewValue(tftypes.Bool, include),
		})
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		r := requests()[0]
		wantQuery := ""
		wantStatusTags := `[]`
		if include {
			wantQuery = "include=tags"
			wantStatusTags = `["VERIFICATION_REQUIRED"]`
		}
		if r.Path != "/v1/domains/example.com" || r.Query.Encode() != wantQuery {
			t.Errorf("include=%v: request = %s?%s, want /v1/domains/example.com?%s", include, r.Path, r.Query.Encode(), wantQuery)
		}
		var state DomainDataSourceModel
		resp.Diagnostics.Append(resp.State.Get(context.Background(), &state)...)
		if got := state.StatusTags.String(); got != wantStatusTags {
			t.Errorf("include=%v: status_tags = %s, want %s", include, got, wantStatusTags)
		}
	}
}

func TestDomainResource_CreateSendsPremiumPriceAndClaimsHash(t *testing.T) {
	client, requests := newRecordingClient(t, func(recordedRequest) (int, string) { return 201, domainJSON })
	r := &DomainResource{client: client}
	ctx := context.Background()

	var sresp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sresp)
	typ := sresp.Schema.Type().TerraformType(ctx)

	contacts := tftypes.NewValue(tftypes.Map{ElementType: tftypes.List{ElementType: tftypes.String}}, map[string]tftypes.Value{
		"registrant": tfStringList("contact_01h45ytscbebyvny4gc8cr8ma2"),
	})
	plan := objectWithOverrides(t, typ, map[string]tftypes.Value{
		"name":                          tftypes.NewValue(tftypes.String, "example.com"),
		"period_value":                  tftypes.NewValue(tftypes.Number, 2),
		"period_unit":                   tftypes.NewValue(tftypes.String, "y"),
		"create_zone":                   tftypes.NewValue(tftypes.Bool, true),
		"transfer":                      tftypes.NewValue(tftypes.Bool, false),
		"transfer_lock":                 tftypes.NewValue(tftypes.Bool, true),
		"renewal_mode":                  tftypes.NewValue(tftypes.String, "renew"),
		"auth_code":                     tftypes.NewValue(tftypes.String, "s3cret"),
		"expected_price":                tftypes.NewValue(tftypes.Number, 1250),
		"claims_notice_acceptance_hash": tftypes.NewValue(tftypes.String, "hash-123"),
		"contacts":                      contacts,
	})
	resp := resource.CreateResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: tftypes.NewValue(typ, nil)}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: sresp.Schema, Raw: plan}}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("got %d requests, want 1 (transfer_lock already matches the response)", len(reqs))
	}
	if reqs[0].Method != http.MethodPost || reqs[0].Path != "/v1/domains" {
		t.Fatalf("request = %s %s, want POST /v1/domains", reqs[0].Method, reqs[0].Path)
	}
	var body map[string]any
	if err := json.NewDecoder(strings.NewReader(reqs[0].Body)).Decode(&body); err != nil {
		t.Fatalf("decoding body %q: %v", reqs[0].Body, err)
	}
	want := map[string]any{
		"name":                          "example.com",
		"renewal_mode":                  "renew",
		"period":                        map[string]any{"value": float64(2), "unit": "y"},
		"auth_code":                     "s3cret",
		"create_zone":                   true,
		"expected_price":                "1250.00",
		"claims_notice_acceptance_hash": "hash-123",
		"contacts":                      map[string]any{"registrant": []any{map[string]any{"contact_id": "contact_01h45ytscbebyvny4gc8cr8ma2"}}},
	}
	got, _ := json.Marshal(body)
	wantJSON, _ := json.Marshal(want)
	if string(got) != string(wantJSON) {
		t.Errorf("body = %s\nwant   %s", got, wantJSON)
	}
}

func TestDomainResource_CreateOmitsPremiumFieldsWhenUnset(t *testing.T) {
	client, requests := newRecordingClient(t, func(recordedRequest) (int, string) { return 201, domainJSON })
	r := &DomainResource{client: client}
	ctx := context.Background()

	var sresp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sresp)
	typ := sresp.Schema.Type().TerraformType(ctx)
	plan := objectWithOverrides(t, typ, map[string]tftypes.Value{
		"name":          tftypes.NewValue(tftypes.String, "example.com"),
		"period_value":  tftypes.NewValue(tftypes.Number, 1),
		"period_unit":   tftypes.NewValue(tftypes.String, "y"),
		"create_zone":   tftypes.NewValue(tftypes.Bool, false),
		"transfer":      tftypes.NewValue(tftypes.Bool, false),
		"transfer_lock": tftypes.NewValue(tftypes.Bool, true),
		"renewal_mode":  tftypes.NewValue(tftypes.String, "renew"),
		"contacts": tftypes.NewValue(tftypes.Map{ElementType: tftypes.List{ElementType: tftypes.String}}, map[string]tftypes.Value{
			"registrant": tfStringList("contact_01h45ytscbebyvny4gc8cr8ma2"),
		}),
	})
	resp := resource.CreateResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: tftypes.NewValue(typ, nil)}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: sresp.Schema, Raw: plan}}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	body := requests()[0].Body
	for _, key := range []string{"expected_price", "claims_notice_acceptance_hash", "auth_code", "attributes"} {
		if strings.Contains(body, key) {
			t.Errorf("body %s contains %q, want it omitted", body, key)
		}
	}
}

func TestClaimsNoticeDataSource_Read(t *testing.T) {
	client, requests := newRecordingClient(t, func(recordedRequest) (int, string) {
		return 200, `{"claims_notices": [{
  "claims_key": "key-1",
  "label": "example",
  "claims_notice_acceptance_hash": "hash-1",
  "claims": [],
  "rendered_html": "<p>notice</p>",
  "notice_title": "Title",
  "notice_intro": "Intro",
  "notice_not_exact_match_intro": "Not exact",
  "notice_footer": "Footer",
  "notice_footer_url": "https://example.com/footer"
}]}`
	})
	resp := readDataSourceWithOverrides(t, &ClaimsNoticeDataSource{client: client}, map[string]tftypes.Value{
		"claims_key": tftypes.NewValue(tftypes.String, "key-1"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	r := requests()[0]
	if r.Method != http.MethodPost || r.Path != "/v1/domains/claims-notices" || r.Body != `{"claims_keys":["key-1"]}` {
		t.Errorf("request = %s %s %s, want POST /v1/domains/claims-notices {\"claims_keys\":[\"key-1\"]}", r.Method, r.Path, r.Body)
	}
	var state ClaimsNoticeDataSourceModel
	resp.Diagnostics.Append(resp.State.Get(context.Background(), &state)...)
	if state.ClaimsNoticeAcceptanceHash.ValueString() != "hash-1" || state.Label.ValueString() != "example" ||
		state.RenderedHTML.ValueString() != "<p>notice</p>" || state.NoticeFooterURL.ValueString() != "https://example.com/footer" ||
		state.NoticeNotExactMatchIntro.ValueString() != "Not exact" {
		t.Errorf("unexpected state: %+v", state)
	}
}

func TestClaimsNoticeDataSource_EmptyResultIsError(t *testing.T) {
	client, _ := newRecordingClient(t, func(recordedRequest) (int, string) { return 200, `{"claims_notices": []}` })
	resp := readDataSourceWithOverrides(t, &ClaimsNoticeDataSource{client: client}, map[string]tftypes.Value{
		"claims_key": tftypes.NewValue(tftypes.String, "gone"),
	})
	if !resp.Diagnostics.HasError() || resp.Diagnostics.Errors()[0].Summary() != "No claims notice returned" {
		t.Fatalf("diagnostics = %v, want \"No claims notice returned\"", resp.Diagnostics)
	}
}

func TestDomainCheckDataSource_Read(t *testing.T) {
	client, requests := newRecordingClient(t, func(recordedRequest) (int, string) {
		return 200, `{"results": [
  {"domain": "free.com", "available": true},
  {"domain": "premium.com", "available": true, "is_premium": true, "claims_key": "ck-1",
   "premium_pricing": {"prices": [{"action": "create", "price": "1250.00", "currency": "USD"}]}},
  {"domain": "taken.com", "available": false, "reason": "registered", "is_premium": false}
]}`
	})
	resp := readDataSourceWithOverrides(t, &DomainCheckDataSource{client: client}, map[string]tftypes.Value{
		"domains": tfStringList("free.com", "premium.com", "taken.com"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	r := requests()[0]
	want := url.Values{"domains": {"free.com", "premium.com", "taken.com"}}
	if r.Method != http.MethodGet || r.Path != "/v1/domains/check" || r.Query.Encode() != want.Encode() {
		t.Errorf("request = %s %s?%s, want GET /v1/domains/check?%s", r.Method, r.Path, r.Query.Encode(), want.Encode())
	}

	var state DomainCheckDataSourceModel
	resp.Diagnostics.Append(resp.State.Get(context.Background(), &state)...)
	results := state.Results.Elements()
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	free := results[0].(types.Object).Attributes()
	if free["is_premium"] != types.BoolValue(false) || !free["claims_key"].IsNull() || !free["reason"].IsNull() {
		t.Errorf("free.com = %v", free)
	}
	premium := results[1].(types.Object).Attributes()
	if premium["is_premium"] != types.BoolValue(true) || premium["claims_key"] != types.StringValue("ck-1") {
		t.Errorf("premium.com = %v", premium)
	}
	prices := premium["premium_pricing"].(types.List).Elements()
	if len(prices) != 1 || prices[0].(types.Object).Attributes()["price"] != types.StringValue("1250.00") {
		t.Errorf("premium_pricing = %v", premium["premium_pricing"])
	}
	taken := results[2].(types.Object).Attributes()
	if taken["reason"] != types.StringValue("registered") {
		t.Errorf("taken.com = %v", taken)
	}
}
