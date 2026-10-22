package provider

import (
	"strings"
	"testing"
	"time"

	"github.com/opusdns/opusdns-go-client/models"
)

func TestFindTLDInfo(t *testing.T) {
	t.Parallel()

	infos := []models.TLDInfo{
		{Name: "co.uk", Type: models.TLDTypeCCTLD},
		{Name: "uk", Type: models.TLDTypeCCTLD},
	}
	if got := findTLDInfo(infos, ".UK"); got == nil || got.Name != "uk" {
		t.Errorf("findTLDInfo(.UK) = %+v, want uk", got)
	}
	if got := findTLDInfo(infos, "org"); got != nil {
		t.Errorf("findTLDInfo(org) = %+v, want nil for an ambiguous specification", got)
	}
	single := []models.TLDInfo{{Name: "com", Type: models.TLDTypeGTLD}}
	if got := findTLDInfo(single, "org"); got == nil || got.Name != "com" {
		t.Errorf("findTLDInfo(single) = %+v, want the only entry", got)
	}
}

func TestBuildTLDPeriodList(t *testing.T) {
	t.Parallel()

	list, diags := buildTLDPeriodList([]models.DomainPeriod{
		{Value: 1, Unit: models.PeriodUnitYear},
		{Value: 6, Unit: models.PeriodUnitMonth},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := len(list.Elements()); got != 2 {
		t.Fatalf("len = %d, want 2", got)
	}

	empty, diags := buildTLDPeriodList(nil)
	if diags.HasError() || empty.IsNull() || len(empty.Elements()) != 0 {
		t.Errorf("nil periods should give an empty, non-null list; got %v (%v)", empty, diags)
	}
}

func TestBuildTLDLocalPresenceObject(t *testing.T) {
	t.Parallel()

	obj, diags := buildTLDLocalPresenceObject(nil)
	if diags.HasError() || !obj.IsNull() {
		t.Errorf("nil local presence should give a null object; got %v (%v)", obj, diags)
	}

	obj, diags = buildTLDLocalPresenceObject(&models.TLDLocalPresence{
		Required:          true,
		EligibleCountries: []string{"DE", "AT"},
	})
	if diags.HasError() || obj.IsNull() {
		t.Fatalf("unexpected result: %v (%v)", obj, diags)
	}
}

func TestBuildTLDContactConfigList_RequiredFromMin(t *testing.T) {
	t.Parallel()

	list, diags := buildTLDContactConfigList([]models.TLDContactRole{
		{Type: models.DomainContactType("registrant"), Min: 1, Max: 1},
		{Type: models.DomainContactType("tech"), Min: 0, Max: 3},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	elems := list.Elements()
	if len(elems) != 2 {
		t.Fatalf("len = %d, want 2", len(elems))
	}
	registrant := elems[0].String()
	tech := elems[1].String()
	if want := `"required":true`; !strings.Contains(registrant, want) {
		t.Errorf("registrant = %s, want it to contain %s", registrant, want)
	}
	if want := `"required":false`; !strings.Contains(tech, want) {
		t.Errorf("tech = %s, want it to contain %s", tech, want)
	}
}

func TestBuildTLDPhasesList(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	alloc := models.AllocationMethodFCFS
	smd := true
	list, diags := buildTLDPhasesList([]models.TLDLaunchPhase{
		{Type: models.LaunchPhaseType("sunrise"), Supported: true, Allocation: &alloc, SMDRequired: &smd, StartDate: &start},
		{Type: models.LaunchPhaseType("landrush"), Supported: false},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := len(list.Elements()); got != 2 {
		t.Fatalf("len = %d, want 2", got)
	}
	if first := list.Elements()[0].String(); !strings.Contains(first, `"start_date":"2026-03-01T12:00:00Z"`) {
		t.Errorf("first phase = %s, want an RFC3339 start_date", first)
	}
	if second := list.Elements()[1].String(); !strings.Contains(second, `"start_date":<null>`) {
		t.Errorf("second phase = %s, want a null start_date", second)
	}
}
