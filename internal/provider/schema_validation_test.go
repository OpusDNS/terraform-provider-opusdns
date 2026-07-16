package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// resourceSchema is a small helper that invokes a resource's Schema method and
// returns the resulting schema for assertions. It requires no API client.
func resourceSchema(t *testing.T, r resource.Resource) rschema.Schema {
	t.Helper()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", resp.Diagnostics)
	}
	return resp.Schema
}

// dataSourceSchema invokes a data source's Schema method and returns the result.
func dataSourceSchema(t *testing.T, d datasource.DataSource) dsschema.Schema {
	t.Helper()
	resp := &datasource.SchemaResponse{}
	d.Schema(context.Background(), datasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", resp.Diagnostics)
	}
	return resp.Schema
}

func TestVanityNameserverSetResourceSchema(t *testing.T) {
	s := resourceSchema(t, NewVanityNameserverSetResource())

	required := []string{"name", "parent_domain_name", "soa_rname", "hostnames"}
	for _, name := range required {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("expected attribute %q to exist", name)
		}
		if !attr.IsRequired() {
			t.Errorf("expected attribute %q to be required", name)
		}
	}

	computed := []string{"id", "set_id", "organization_id", "status", "nameservers"}
	for _, name := range computed {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("expected attribute %q to exist", name)
		}
		if !attr.IsComputed() {
			t.Errorf("expected attribute %q to be computed", name)
		}
	}

	isDefault, ok := s.Attributes["is_default"]
	if !ok {
		t.Fatal("expected attribute is_default to exist")
	}
	if !isDefault.IsOptional() || !isDefault.IsComputed() {
		t.Error("expected is_default to be optional+computed")
	}
}

func TestVanityNameserverSetDataSourceSchema(t *testing.T) {
	s := dataSourceSchema(t, NewVanityNameserverSetDataSource())
	if attr, ok := s.Attributes["set_id"]; !ok || !attr.IsRequired() {
		t.Error("expected set_id to be a required attribute")
	}
	if _, ok := s.Attributes["nameservers"]; !ok {
		t.Error("expected nameservers attribute to exist")
	}
}

func TestVanityNameserverSetsDataSourceSchema(t *testing.T) {
	s := dataSourceSchema(t, NewVanityNameserverSetsDataSource())
	if _, ok := s.Attributes["sets"]; !ok {
		t.Error("expected sets attribute to exist")
	}
}

func TestRoleResourceSchema(t *testing.T) {
	s := resourceSchema(t, NewRoleResource())

	if attr, ok := s.Attributes["name"]; !ok || !attr.IsRequired() {
		t.Error("expected name to be required")
	}
	if attr, ok := s.Attributes["permissions"]; !ok || !attr.IsRequired() {
		t.Error("expected permissions to be required")
	}
	if attr, ok := s.Attributes["description"]; !ok || !attr.IsOptional() {
		t.Error("expected description to be optional")
	}
	for _, name := range []string{"id", "label", "built_in", "created_on", "updated_on"} {
		if attr, ok := s.Attributes[name]; !ok || !attr.IsComputed() {
			t.Errorf("expected %q to be computed", name)
		}
	}
}

func TestRolePermissionsDataSourceSchema(t *testing.T) {
	s := dataSourceSchema(t, NewRolePermissionsDataSource())
	if attr, ok := s.Attributes["permissions"]; !ok || !attr.IsComputed() {
		t.Error("expected permissions to be a computed attribute")
	}
}

func TestZoneResourceVanitySchema(t *testing.T) {
	s := resourceSchema(t, NewZoneResource())
	attr, ok := s.Attributes["vanity_nameserver_set_id"]
	if !ok {
		t.Fatal("expected vanity_nameserver_set_id attribute to exist")
	}
	if !attr.IsOptional() || !attr.IsComputed() {
		t.Error("expected vanity_nameserver_set_id to be optional+computed")
	}
}

func TestDomainResourceRegistrationSchema(t *testing.T) {
	s := resourceSchema(t, NewDomainResource())

	// contacts is now optional (to permit zero-contact transfers).
	if attr, ok := s.Attributes["contacts"]; !ok || attr.IsRequired() {
		t.Error("expected contacts to be optional, not required")
	}
	if attr, ok := s.Attributes["expected_price"]; !ok || !attr.IsOptional() {
		t.Error("expected expected_price to be an optional attribute")
	}
	if attr, ok := s.Attributes["claims_notice_acceptance_hash"]; !ok || !attr.IsOptional() {
		t.Error("expected claims_notice_acceptance_hash to be an optional attribute")
	}
}

func TestDomainsDataSourceStatusTagsSchema(t *testing.T) {
	s := dataSourceSchema(t, NewDomainsDataSource())
	if attr, ok := s.Attributes["status_tags"]; !ok || !attr.IsOptional() {
		t.Error("expected status_tags filter to be optional")
	}
	if attr, ok := s.Attributes["status_tag_mode"]; !ok || !attr.IsOptional() {
		t.Error("expected status_tag_mode filter to be optional")
	}
}
