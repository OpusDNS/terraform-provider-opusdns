package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
)

func TestFilterTLDs(t *testing.T) {
	t.Parallel()

	tlds := []models.TLD{
		{Name: "com", Type: models.TLDTypeGTLD, Available: true, DNSSECSupported: true},
		{Name: "de", Type: models.TLDTypeCCTLD, Available: true, DNSSECSupported: true},
		{Name: "dev", Type: models.TLDTypeGTLD, Available: true},
	}
	unfiltered := TLDsDataSourceModel{
		Search:              types.StringNull(),
		Type:                types.StringNull(),
		Available:           types.BoolNull(),
		RegistrationEnabled: types.BoolNull(),
		DNSSECSupported:     types.BoolNull(),
	}

	tests := []struct {
		name string
		edit func(*TLDsDataSourceModel)
		want []string
	}{
		{"no filters", func(*TLDsDataSourceModel) {}, []string{"com", "de", "dev"}},
		{"search", func(m *TLDsDataSourceModel) { m.Search = types.StringValue("DE") }, []string{"de", "dev"}},
		{"type", func(m *TLDsDataSourceModel) { m.Type = types.StringValue("ccTLD") }, []string{"de"}},
		{"dnssec supported", func(m *TLDsDataSourceModel) { m.DNSSECSupported = types.BoolValue(true) }, []string{"com", "de"}},
		{"dnssec not supported", func(m *TLDsDataSourceModel) { m.DNSSECSupported = types.BoolValue(false) }, []string{"dev"}},
		{"available false", func(m *TLDsDataSourceModel) { m.Available = types.BoolValue(false) }, nil},
		// The API has no registration-enabled flag, so the filter is ignored
		// instead of matching nothing.
		{"registration enabled ignored", func(m *TLDsDataSourceModel) { m.RegistrationEnabled = types.BoolValue(true) }, []string{"com", "de", "dev"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := unfiltered
			tt.edit(&data)
			got := filterTLDs(tlds, data)
			if len(got) != len(tt.want) {
				t.Fatalf("filterTLDs() returned %d TLDs, want %v", len(got), tt.want)
			}
			for i, name := range tt.want {
				if got[i].Name != name {
					t.Errorf("filterTLDs()[%d] = %q, want %q", i, got[i].Name, name)
				}
			}
		})
	}
}

func TestRegistrationYearsValue(t *testing.T) {
	t.Parallel()

	if got := registrationYearsValue(0); !got.IsNull() {
		t.Errorf("registrationYearsValue(0) = %v, want null", got)
	}
	if got := registrationYearsValue(10); got.ValueInt64() != 10 {
		t.Errorf("registrationYearsValue(10) = %v, want 10", got)
	}
}
