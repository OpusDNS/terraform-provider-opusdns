package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
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

func TestBuildDomainListQuery_StatusFilters(t *testing.T) {
	renewal := models.RenewalModeRenew
	opts := &models.ListDomainsOptions{
		Search:      "example",
		TLD:         "com",
		RenewalMode: &renewal,
		Status:      models.DomainStatus("ok"),
	}
	q := buildDomainListQuery(opts)
	if got := q.Get("search"); got != "example" {
		t.Errorf("search = %q, want example", got)
	}
	if got := q.Get("tld"); got != "com" {
		t.Errorf("tld = %q, want com", got)
	}
	if got := q.Get("renewal_mode"); got != "renew" {
		t.Errorf("renewal_mode = %q, want renew", got)
	}
	if got := q.Get("status"); got != "ok" {
		t.Errorf("status = %q, want ok", got)
	}
}

func TestDomainCreateRequestToMap_AddsExtraFields(t *testing.T) {
	req := &models.DomainCreateRequest{
		Name:        "example.com",
		RenewalMode: models.RenewalModeRenew,
		Period:      models.DomainPeriod{Value: 1, Unit: models.PeriodUnitYear},
	}
	body, err := domainCreateRequestToMap(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["name"] != "example.com" {
		t.Errorf("name = %v, want example.com", body["name"])
	}
	// Confirm extra create-only fields can be layered on.
	body["expected_price"] = 12.5
	body["claims_notice_acceptance_hash"] = "hash"
	if body["expected_price"] != 12.5 {
		t.Errorf("expected_price not set correctly")
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
