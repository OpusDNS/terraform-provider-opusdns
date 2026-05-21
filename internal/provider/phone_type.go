package provider

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// phoneType is a custom Terraform string type for phone/fax numbers whose
// values the OpusDNS API may rewrite to a canonical form (e.g.
// "+1.2125551234" -> "+1 212-555-1234"). It implements semantic equality so
// that the plugin framework accepts a post-apply state value that differs
// from the planned value as long as both forms canonicalise to the same
// digits.
type phoneType struct {
	basetypes.StringType
}

var (
	_ basetypes.StringTypable = phoneType{}
)

func (phoneType) Equal(o attr.Type) bool {
	_, ok := o.(phoneType)
	return ok
}

func (phoneType) String() string { return "phoneType" }

func (t phoneType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return phoneValue{StringValue: in}, nil
}

func (t phoneType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	sv, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	str, ok := sv.(basetypes.StringValue)
	if !ok {
		return nil, nil
	}
	return phoneValue{StringValue: str}, nil
}

func (phoneType) ValueType(_ context.Context) attr.Value {
	return phoneValue{}
}

// phoneValue is the value type for phoneType. Its StringSemanticEquals
// implementation strips all non-digit characters (except a leading "+") and
// compares the canonical forms.
type phoneValue struct {
	basetypes.StringValue
}

var (
	_ basetypes.StringValuableWithSemanticEquals = phoneValue{}
)

func (v phoneValue) Type(_ context.Context) attr.Type {
	return phoneType{}
}

func (v phoneValue) Equal(o attr.Value) bool {
	other, ok := o.(phoneValue)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

var phoneNonDigit = regexp.MustCompile(`[^\d]`)

// canonicalPhone returns the digits-only form of a phone number, preserving
// a leading "+" so country-code presence is significant.
func canonicalPhone(s string) string {
	plus := ""
	if len(s) > 0 && s[0] == '+' {
		plus = "+"
	}
	return plus + phoneNonDigit.ReplaceAllString(s, "")
}

func normalizedPhoneValue(s *string) phoneValue {
	if s == nil {
		return phoneValue{StringValue: types.StringNull()}
	}
	return phoneValue{StringValue: types.StringValue(canonicalPhone(*s))}
}

func (v phoneValue) StringSemanticEquals(_ context.Context, other basetypes.StringValuable) (bool, diag.Diagnostics) {
	otherPhone, ok := other.(phoneValue)
	if !ok {
		return false, nil
	}
	if v.IsNull() || v.IsUnknown() || otherPhone.IsNull() || otherPhone.IsUnknown() {
		return v.StringValue.Equal(otherPhone.StringValue), nil
	}
	return canonicalPhone(v.ValueString()) == canonicalPhone(otherPhone.ValueString()), nil
}
