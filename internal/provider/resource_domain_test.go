package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/opusdns/opusdns-go-client/models"
)

// testContactsMap builds a minimal, valid contacts map value (a single
// registrant) for exercising validateDomainConfig's registration rules.
func testContactsMap(t *testing.T) types.Map {
	t.Helper()
	registrant, d := types.ListValue(types.StringType, []attr.Value{types.StringValue("contact_1")})
	if d.HasError() {
		t.Fatalf("building registrant list: %v", d)
	}
	m, d := types.MapValue(contactsMapElemType, map[string]attr.Value{"registrant": registrant})
	if d.HasError() {
		t.Fatalf("building contacts map: %v", d)
	}
	return m
}

func TestAccDomainResource_basic(t *testing.T) {
	domainLabel := testAccDomainLabel()
	domainName := testAccDomainNameFromLabel(domainLabel)
	contactKey := fmt.Sprintf("tfacc-%d", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDomainResourceConfigBasic(domainName, contactKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "id"),
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "domain_id"),
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "owner_id"),
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "registry_account_id"),
					resource.TestCheckResourceAttr("opusdns_domain.test", "name", domainName),
					resource.TestCheckResourceAttr("opusdns_domain.test", "sld", domainLabel),
					resource.TestCheckResourceAttr("opusdns_domain.test", "tld", testAccDomainTLD),
					resource.TestCheckResourceAttr("opusdns_domain.test", "period_value", "1"),
					resource.TestCheckResourceAttr("opusdns_domain.test", "period_unit", "y"),
					resource.TestCheckResourceAttr("opusdns_domain.test", "create_zone", "true"),
					resource.TestCheckResourceAttr("opusdns_domain.test", "renewal_mode", "renew"),
					resource.TestCheckResourceAttr("opusdns_domain.test", "transfer_lock", "true"),
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "registered_on"),
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "expires_on"),
				),
			},
		},
	})
}

func TestAccDomainResource_updateTransferLock(t *testing.T) {
	domainName := testAccDomainName()
	contactKey := fmt.Sprintf("tfacc-%d", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDomainResourceConfigTransferLock(domainName, contactKey, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_domain.test", "name", domainName),
					resource.TestCheckResourceAttr("opusdns_domain.test", "transfer_lock", "false"),
				),
			},
			{
				Config: testAccDomainResourceConfigTransferLock(domainName, contactKey, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_domain.test", "name", domainName),
					resource.TestCheckResourceAttr("opusdns_domain.test", "transfer_lock", "true"),
				),
			},
		},
	})
}

func testAccDomainResourceConfigBasic(domainName, contactKey string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_contact" "test" {
  first_name  = "Terraform"
  last_name   = "Acceptance"
  org         = "OpusDNS"
  email       = "%s@example.com"
  phone       = "+1.2125551234"
  street      = "123 Terraform Street"
  city        = "Exampleville"
  state       = "NY"
  postal_code = "10001"
  country     = "US"
}

resource "opusdns_domain" "test" {
  name         = %q
  period_value = 1
  period_unit  = "y"
  create_zone  = true

  contacts = {
    registrant = [opusdns_contact.test.contact_id]
    admin      = [opusdns_contact.test.contact_id]
    tech       = [opusdns_contact.test.contact_id]
  }
}
`, testAccProviderConfig, contactKey, domainName)
}

func testAccDomainResourceConfigTransferLock(domainName, contactKey string, transferLock bool) string {
	return fmt.Sprintf(`
%s

resource "opusdns_contact" "test" {
  first_name  = "Terraform"
  last_name   = "Acceptance"
  org         = "OpusDNS"
  email       = "%s@example.com"
  phone       = "+1.2125551234"
  street      = "123 Terraform Street"
  city        = "Exampleville"
  state       = "NY"
  postal_code = "10001"
  country     = "US"
}

resource "opusdns_domain" "test" {
  name          = %q
  period_value  = 1
  period_unit   = "y"
  create_zone   = true
  transfer_lock = %t

  contacts = {
    registrant = [opusdns_contact.test.contact_id]
    admin      = [opusdns_contact.test.contact_id]
    tech       = [opusdns_contact.test.contact_id]
  }
}
`, testAccProviderConfig, contactKey, domainName, transferLock)
}

// TestAccDomainResource_transfer exercises the transfer-in create path
// (POST /v1/domains/transfer via `transfer = true`). It is skipped by default:
// a real transfer requires a domain already registered at a losing registrar
// plus a valid EPP auth code, which CI cannot provision on demand. Set
// OPUSDNS_ACC_TRANSFER_DOMAIN and OPUSDNS_ACC_TRANSFER_AUTH_CODE (and unskip)
// to run it against a prepared fixture. Mirrors the skip convention used by
// TestAccRecordResource_NS.
func TestAccDomainResource_transfer(t *testing.T) {
	t.Skip("skipped by default: domain transfer-in needs a domain at a losing registrar plus a valid auth code; see docs/plans/domain-transfer-in.md")

	domainName := testAccDomainName()
	contactKey := fmt.Sprintf("tfacc-%d", rand.Int63())
	authCode := "REPLACE-WITH-REAL-AUTH-CODE"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDomainResourceConfigTransfer(domainName, contactKey, authCode),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_domain.test", "name", domainName),
					resource.TestCheckResourceAttr("opusdns_domain.test", "transfer", "true"),
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "domain_id"),
				),
			},
		},
	})
}

func testAccDomainResourceConfigTransfer(domainName, contactKey, authCode string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_contact" "test" {
  first_name  = "Terraform"
  last_name   = "Acceptance"
  org         = "OpusDNS"
  email       = "%s@example.com"
  phone       = "+1.2125551234"
  street      = "123 Terraform Street"
  city        = "Exampleville"
  state       = "NY"
  postal_code = "10001"
  country     = "US"
}

resource "opusdns_domain" "test" {
  name         = %q
  transfer     = true
  auth_code    = %q
  period_value = 1
  period_unit  = "y"

  contacts = {
    registrant = [opusdns_contact.test.contact_id]
    admin      = [opusdns_contact.test.contact_id]
    tech       = [opusdns_contact.test.contact_id]
  }
}
`, testAccProviderConfig, contactKey, domainName, authCode)
}

// --- unit tests (no API) ---

func TestValidateDomainConfig(t *testing.T) {
	t.Parallel()

	base := func() DomainResourceModel {
		return DomainResourceModel{
			Name:        types.StringValue("example.com"),
			Transfer:    types.BoolValue(true),
			AuthCode:    types.StringValue("ABC-123"),
			CreateZone:  types.BoolValue(false),
			PeriodUnit:  types.StringValue("y"),
			PeriodValue: types.Int64Value(1),
		}
	}

	tests := []struct {
		name      string
		mutate    func(*DomainResourceModel)
		wantError bool
	}{
		{
			name: "register path with contacts skips transfer rules",
			mutate: func(m *DomainResourceModel) {
				m.Transfer = types.BoolValue(false)
				m.AuthCode = types.StringNull()
				m.Contacts = testContactsMap(t)
			},
			wantError: false,
		},
		{
			name:      "register path missing contacts",
			mutate:    func(m *DomainResourceModel) { m.Transfer = types.BoolValue(false); m.AuthCode = types.StringNull() },
			wantError: true,
		},
		{
			name:      "valid transfer",
			mutate:    func(_ *DomainResourceModel) {},
			wantError: false,
		},
		{
			name:      "transfer missing auth_code (null)",
			mutate:    func(m *DomainResourceModel) { m.AuthCode = types.StringNull() },
			wantError: true,
		},
		{
			name:      "transfer empty auth_code",
			mutate:    func(m *DomainResourceModel) { m.AuthCode = types.StringValue("") },
			wantError: true,
		},
		{
			name:      "transfer with create_zone",
			mutate:    func(m *DomainResourceModel) { m.CreateZone = types.BoolValue(true) },
			wantError: true,
		},
		{
			name:      "transfer with non-year period_unit",
			mutate:    func(m *DomainResourceModel) { m.PeriodUnit = types.StringValue("m") },
			wantError: true,
		},
		{
			name:      "transfer with null period_unit is allowed",
			mutate:    func(m *DomainResourceModel) { m.PeriodUnit = types.StringNull() },
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := base()
			tt.mutate(&m)
			diags := validateDomainConfig(m)
			if diags.HasError() != tt.wantError {
				t.Fatalf("validateDomainConfig() error = %v, wantError = %v (diags: %v)", diags.HasError(), tt.wantError, diags)
			}
		})
	}
}

func TestBuildDomainTransferRequest(t *testing.T) {
	t.Parallel()

	data := DomainResourceModel{
		Name:        types.StringValue("example.com"),
		AuthCode:    types.StringValue("ABC-123"),
		RenewalMode: types.StringValue("renew"),
		PeriodValue: types.Int64Value(2),
	}
	contacts := map[models.DomainContactType][]models.ContactHandle{
		models.DomainContactType("registrant"): {{ContactID: models.ContactID("contact_1")}},
	}
	nameservers := []models.Nameserver{{Hostname: "ns1.example.com"}}

	req := buildDomainTransferRequest(data, contacts, nameservers)

	if req.Name != "example.com" {
		t.Errorf("Name = %q, want example.com", req.Name)
	}
	if req.AuthCode != "ABC-123" {
		t.Errorf("AuthCode = %q, want ABC-123", req.AuthCode)
	}
	if req.RenewalMode != models.RenewalMode("renew") {
		t.Errorf("RenewalMode = %q, want renew", req.RenewalMode)
	}
	if req.Period != 2 {
		t.Errorf("Period = %d, want 2", req.Period)
	}
	if len(req.Contacts) != 1 || len(req.Contacts[models.DomainContactType("registrant")]) != 1 {
		t.Errorf("Contacts not mapped through: %#v", req.Contacts)
	}
	if len(req.Nameservers) != 1 || req.Nameservers[0].Hostname != "ns1.example.com" {
		t.Errorf("Nameservers not mapped through: %#v", req.Nameservers)
	}
}
