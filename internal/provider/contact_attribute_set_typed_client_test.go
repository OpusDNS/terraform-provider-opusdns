package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// newTestClient returns a client that talks to handler over httptest.
func newTestClient(t *testing.T, handler http.HandlerFunc) *opusdns.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client, err := opusdns.NewClient(
		opusdns.WithAPIKey("opk_test"),
		opusdns.WithAPIEndpoint(srv.URL),
		opusdns.WithMaxRetries(0),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

// nullObject returns a null-valued attribute map for typ, used as an empty
// config for schemas whose attributes are all optional or computed.
func nullObject(ctx context.Context, t *testing.T, typ tftypes.Type) tftypes.Value {
	t.Helper()
	obj, ok := typ.(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is %T, want object", typ)
	}
	vals := make(map[string]tftypes.Value, len(obj.AttributeTypes))
	for name, attrType := range obj.AttributeTypes {
		vals[name] = tftypes.NewValue(attrType, nil)
	}
	return tftypes.NewValue(obj, vals)
}

func readDataSource(ctx context.Context, t *testing.T, d datasource.DataSource, config func(map[string]tftypes.Value)) (*datasource.ReadResponse, tfsdk.State) {
	t.Helper()
	var sresp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &sresp)
	typ := sresp.Schema.Type().TerraformType(ctx)
	raw := nullObject(ctx, t, typ)
	if config != nil {
		var vals map[string]tftypes.Value
		if err := raw.As(&vals); err != nil {
			t.Fatalf("raw.As: %v", err)
		}
		config(vals)
		raw = tftypes.NewValue(typ, vals)
	}
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: nullObject(ctx, t, typ)}}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: sresp.Schema, Raw: raw}}, resp)
	return resp, resp.State
}

func TestTLDPortfolioDataSourceReadUsesTypedClient(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/tlds/portfolio" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"tld":"com","type":"gTLD"},{"tld":"de","type":"ccTLD"}]`))
	})

	d := &TLDPortfolioDataSource{client: client}
	resp, state := readDataSource(ctx, t, d, nil)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	var model TLDPortfolioDataSourceModel
	if diags := state.Get(ctx, &model); diags.HasError() {
		t.Fatalf("state.Get: %v", diags)
	}
	if model.ID.ValueString() != "tld_portfolio" || model.Total.ValueInt64() != 2 {
		t.Errorf("got id=%q total=%d", model.ID.ValueString(), model.Total.ValueInt64())
	}
	elems := model.TLDs.Elements()
	if len(elems) != 2 {
		t.Fatalf("got %d tlds, want 2", len(elems))
	}
	second := elems[1].(types.Object).Attributes()
	if second["name"].(types.String).ValueString() != "de" || second["type"].(types.String).ValueString() != "ccTLD" {
		t.Errorf("unexpected second entry: %v", second)
	}
}

func TestContactAttributeSetsDataSourceReadPaginatesAndFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	set := func(id string) map[string]interface{} {
		return map[string]interface{}{
			"contact_attribute_set_id": id,
			"organization_id":          "organization_01",
			"label":                    "set " + id,
			"tld":                      "de",
			"attributes":               map[string]string{"k": "v"},
			"linked_contacts":          3,
			"created_on":               "2026-01-02T03:04:05Z",
			"updated_on":               "2026-02-03T04:05:06Z",
		}
	}
	var pages []string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/contacts/attribute-sets" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		q := r.URL.Query()
		pages = append(pages, q.Get("page"))
		if q.Get("tld") != "de" || q.Get("label") != "main" || q.Get("page_size") != "100" {
			t.Errorf("unexpected query %v", q)
		}
		hasNext := q.Get("page") == "1"
		id := "cas_2"
		if hasNext {
			id = "cas_1"
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"results":    []interface{}{set(id)},
			"pagination": map[string]interface{}{"total_count": 2, "total_pages": 2, "current_page": len(pages), "page_size": 100, "has_next_page": hasNext, "has_previous_page": !hasNext},
		})
	})

	d := &ContactAttributeSetsDataSource{client: client}
	resp, state := readDataSource(ctx, t, d, func(vals map[string]tftypes.Value) {
		vals["tld"] = tftypes.NewValue(tftypes.String, "de")
		vals["label"] = tftypes.NewValue(tftypes.String, "main")
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if strings.Join(pages, ",") != "1,2" {
		t.Errorf("requested pages %v, want 1,2", pages)
	}
	var model ContactAttributeSetsDataSourceModel
	if diags := state.Get(ctx, &model); diags.HasError() {
		t.Fatalf("state.Get: %v", diags)
	}
	elems := model.ContactAttributeSets.Elements()
	if len(elems) != 2 {
		t.Fatalf("got %d sets, want 2", len(elems))
	}
	first := elems[0].(types.Object).Attributes()
	if got := first["contact_attribute_set_id"].(types.String).ValueString(); got != "cas_1" {
		t.Errorf("first id = %q", got)
	}
	if got := first["linked_contacts"].(types.Int64).ValueInt64(); got != 3 {
		t.Errorf("linked_contacts = %d", got)
	}
	if got := first["created_on"].(types.String).ValueString(); got != "2026-01-02T03:04:05Z" {
		t.Errorf("created_on = %q", got)
	}
}

func TestContactAttributeLinkResourceCreateAndRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/contacts/contact_1/link/cas_1":
			_, _ = w.Write([]byte(`{"contact_attribute_link_id":"cal_1","contact_id":"contact_1","contact_attribute_set_id":"cas_1","tld":"de","created_on":"2026-01-02T03:04:05Z"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/contacts/contact_1":
			_, _ = w.Write([]byte(`{"contact_id":"contact_1","attribute_sets":[{"contact_attribute_set_id":"cas_0","label":"x","tld":"fr","attributes":{}},{"contact_attribute_set_id":"cas_1","label":"y","tld":"de","attributes":{"k":"v"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/contacts/contact_gone":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"not found"}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	r := &ContactAttributeLinkResource{client: client}
	var sresp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sresp)
	typ := sresp.Schema.Type().TerraformType(ctx)

	withIDs := func(contactID string) tftypes.Value {
		var vals map[string]tftypes.Value
		if err := nullObject(ctx, t, typ).As(&vals); err != nil {
			t.Fatal(err)
		}
		vals["contact_id"] = tftypes.NewValue(tftypes.String, contactID)
		vals["contact_attribute_set_id"] = tftypes.NewValue(tftypes.String, "cas_1")
		return tftypes.NewValue(typ, vals)
	}

	cresp := &resource.CreateResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: nullObject(ctx, t, typ)}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: sresp.Schema, Raw: withIDs("contact_1")}}, cresp)
	if cresp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", cresp.Diagnostics)
	}
	var id, created types.String
	cresp.State.GetAttribute(ctx, path.Root("contact_attribute_link_id"), &id)
	cresp.State.GetAttribute(ctx, path.Root("created_on"), &created)
	if id.ValueString() != "cal_1" || created.ValueString() != "2026-01-02T03:04:05Z" {
		t.Errorf("create state: id=%q created_on=%q", id.ValueString(), created.ValueString())
	}

	rresp := &resource.ReadResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: withIDs("contact_1")}}
	r.Read(ctx, resource.ReadRequest{State: rresp.State}, rresp)
	if rresp.Diagnostics.HasError() {
		t.Fatalf("read diagnostics: %v", rresp.Diagnostics)
	}
	var tld types.String
	rresp.State.GetAttribute(ctx, path.Root("tld"), &tld)
	if tld.ValueString() != "de" {
		t.Errorf("tld = %q, want de", tld.ValueString())
	}

	gresp := &resource.ReadResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: withIDs("contact_gone")}}
	r.Read(ctx, resource.ReadRequest{State: gresp.State}, gresp)
	if gresp.Diagnostics.HasError() {
		t.Fatalf("read of a missing contact must not error: %v", gresp.Diagnostics)
	}
	if !gresp.State.Raw.IsNull() {
		t.Error("state must be removed when the contact is gone")
	}
}

func TestContactAttributeSetResourceCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var gotCreate, gotPatch map[string]interface{}
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := func() map[string]interface{} {
			var m map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&m)
			return m
		}
		resp := `{"contact_attribute_set_id":"cas_1","organization_id":"organization_1","label":"main","tld":"de","attributes":{"a":"b"},"linked_contacts":0,"created_on":"2026-01-02T03:04:05Z","updated_on":"2026-01-02T03:04:05Z"}`
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/contacts/attribute-sets":
			gotCreate = body()
			_, _ = w.Write([]byte(resp))
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/contacts/attribute-sets/cas_1":
			gotPatch = body()
			_, _ = w.Write([]byte(strings.Replace(resp, `"main"`, `"renamed"`, 1)))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/contacts/attribute-sets/cas_1":
			_, _ = w.Write([]byte(resp))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/contacts/attribute-sets/cas_gone":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"not found"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/contacts/attribute-sets/cas_1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	r := &ContactAttributeSetResource{client: client}
	var sresp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sresp)
	typ := sresp.Schema.Type().TerraformType(ctx)
	mapType := tftypes.Map{ElementType: tftypes.String}

	build := func(set func(map[string]tftypes.Value)) tftypes.Value {
		var vals map[string]tftypes.Value
		if err := nullObject(ctx, t, typ).As(&vals); err != nil {
			t.Fatal(err)
		}
		set(vals)
		return tftypes.NewValue(typ, vals)
	}
	plan := build(func(v map[string]tftypes.Value) {
		v["label"] = tftypes.NewValue(tftypes.String, "main")
		v["tld"] = tftypes.NewValue(tftypes.String, "de")
		v["attributes"] = tftypes.NewValue(mapType, map[string]tftypes.Value{"a": tftypes.NewValue(tftypes.String, "b")})
	})

	cresp := &resource.CreateResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: nullObject(ctx, t, typ)}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: sresp.Schema, Raw: plan}}, cresp)
	if cresp.Diagnostics.HasError() {
		t.Fatalf("create diagnostics: %v", cresp.Diagnostics)
	}
	if gotCreate["label"] != "main" || gotCreate["tld"] != "de" || gotCreate["attributes"].(map[string]interface{})["a"] != "b" {
		t.Errorf("create body = %v", gotCreate)
	}
	var updatedOn types.String
	cresp.State.GetAttribute(ctx, path.Root("updated_on"), &updatedOn)
	if updatedOn.ValueString() != "2026-01-02T03:04:05Z" {
		t.Errorf("updated_on = %q", updatedOn.ValueString())
	}

	// Update with a changed label sends only the label.
	newPlan := build(func(v map[string]tftypes.Value) {
		v["label"] = tftypes.NewValue(tftypes.String, "renamed")
		v["tld"] = tftypes.NewValue(tftypes.String, "de")
		v["attributes"] = tftypes.NewValue(mapType, map[string]tftypes.Value{"a": tftypes.NewValue(tftypes.String, "b")})
	})
	uresp := &resource.UpdateResponse{State: cresp.State}
	r.Update(ctx, resource.UpdateRequest{Plan: tfsdk.Plan{Schema: sresp.Schema, Raw: newPlan}, State: cresp.State}, uresp)
	if uresp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", uresp.Diagnostics)
	}
	if len(gotPatch) != 1 || gotPatch["label"] != "renamed" {
		t.Errorf("patch body = %v, want only label", gotPatch)
	}

	// Read of a vanished set drops it from state.
	gone := build(func(v map[string]tftypes.Value) {
		v["contact_attribute_set_id"] = tftypes.NewValue(tftypes.String, "cas_gone")
	})
	rresp := &resource.ReadResponse{State: tfsdk.State{Schema: sresp.Schema, Raw: gone}}
	r.Read(ctx, resource.ReadRequest{State: rresp.State}, rresp)
	if rresp.Diagnostics.HasError() || !rresp.State.Raw.IsNull() {
		t.Errorf("read of missing set: diags=%v null=%v", rresp.Diagnostics, rresp.State.Raw.IsNull())
	}

	dresp := &resource.DeleteResponse{}
	r.Delete(ctx, resource.DeleteRequest{State: cresp.State}, dresp)
	if dresp.Diagnostics.HasError() {
		t.Errorf("delete diagnostics: %v", dresp.Diagnostics)
	}
}
