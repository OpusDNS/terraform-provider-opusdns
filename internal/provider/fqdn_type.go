package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// fqdnType is a custom Terraform string type for hostname / FQDN values
// whose canonical form the OpusDNS API rewrites with a trailing dot (e.g.
// user supplies "example.com", server returns "example.com."). It
// implements semantic equality so that the plugin framework accepts a
// post-Read state value that differs from the user-supplied form by a
// single trailing dot.
//
// Canonicalization: trim a single trailing dot. Case is preserved because
// no server-side case folding has been observed in OpusDNS; this can be
// extended to case-insensitive equality later if needed.
type fqdnType struct {
	basetypes.StringType
}

var (
	_ basetypes.StringTypable = fqdnType{}
)

func (fqdnType) Equal(o attr.Type) bool {
	_, ok := o.(fqdnType)
	return ok
}

func (fqdnType) String() string { return "fqdnType" }

func (t fqdnType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return fqdnValue{StringValue: in}, nil
}

func (t fqdnType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	sv, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	str, ok := sv.(basetypes.StringValue)
	if !ok {
		return nil, nil
	}
	return fqdnValue{StringValue: str}, nil
}

func (fqdnType) ValueType(_ context.Context) attr.Value {
	return fqdnValue{}
}

// fqdnValue is the value type for fqdnType. Its StringSemanticEquals
// implementation strips a single trailing dot from both sides and
// compares the results.
type fqdnValue struct {
	basetypes.StringValue
}

var (
	_ basetypes.StringValuableWithSemanticEquals = fqdnValue{}
)

func (v fqdnValue) Type(_ context.Context) attr.Type {
	return fqdnType{}
}

func (v fqdnValue) Equal(o attr.Value) bool {
	other, ok := o.(fqdnValue)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

// canonicalFQDN returns the hostname with a single trailing dot removed,
// if present. Case is preserved.
func canonicalFQDN(s string) string {
	return strings.TrimSuffix(s, ".")
}

func (v fqdnValue) StringSemanticEquals(_ context.Context, other basetypes.StringValuable) (bool, diag.Diagnostics) {
	otherFQDN, ok := other.(fqdnValue)
	if !ok {
		return false, nil
	}
	if v.IsNull() || v.IsUnknown() || otherFQDN.IsNull() || otherFQDN.IsUnknown() {
		return v.StringValue.Equal(otherFQDN.StringValue), nil
	}
	return canonicalFQDN(v.ValueString()) == canonicalFQDN(otherFQDN.ValueString()), nil
}
