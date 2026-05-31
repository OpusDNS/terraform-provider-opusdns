package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// isNotFound returns true when an API error indicates a 404 Not Found response.
func isNotFound(err error) bool {
	return errors.Is(err, opusdns.ErrNotFound)
}

// optionalStringPtr returns a *string for a types.String, or nil when null/unknown.
// Useful for SDK request types whose optional fields are *string.
func optionalStringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// mapToStringMap converts a Terraform `map(string)` into a plain
// `map[string]string`, returning an empty map for null/unknown inputs.
func mapToStringMap(ctx context.Context, m types.Map) (map[string]string, diag.Diagnostics) {
	out := map[string]string{}
	if m.IsNull() || m.IsUnknown() {
		return out, nil
	}
	d := m.ElementsAs(ctx, &out, false)
	return out, d
}

// stringPtrToValue converts a *string (typical of SDK response types) into a
// types.String, mapping nil to types.StringNull().
func stringPtrToValue(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// trimTrailingDot removes a single trailing `.` from a hostname. The OpusDNS
// API returns hostnames in fully-qualified form with a trailing dot (e.g.
// `example.com.`), but Terraform users supply, and naturally compare, the
// dot-less form. Strip on the way into state so plan-vs-state diffs and
// post-apply consistency checks align.
func trimTrailingDot(s string) string {
	return strings.TrimSuffix(s, ".")
}

// stringSlicesEqual reports whether two []string have the same length and
// element-wise equal contents in the same order. Used to short-circuit
// no-op API calls when comparing plan-vs-state list values whose
// ordering is semantically significant (e.g. `forward_to`).
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// fqdnRDataTypes is the set of DNS record types whose rdata wire form
// is, or ends in, a fully-qualified domain name. The OpusDNS API returns
// these in trailing-dot form (e.g. `target.example.com.`) while users
// typically write the dot-less form in their Terraform config. Stripping
// the trailing dot on the way into state keeps plan-vs-state diffs and
// post-apply consistency checks aligned.
//
// For MX and SRV the rdata is a space-separated tuple whose last token
// is the hostname; we strip only that token's trailing dot.
var fqdnRDataTypes = map[string]bool{
	"CNAME": true,
	"DNAME": true,
	"MX":    true,
	"NS":    true,
	"PTR":   true,
	"SRV":   true,
}

// normalizeRData strips a trailing `.` from FQDN-bearing record rdata so
// values returned by the API compare equal to dot-less values supplied
// by users. Types not listed in fqdnRDataTypes are returned unchanged.
// For composite types (MX, SRV) only the trailing dot of the final
// whitespace-separated token (the hostname) is removed.
func normalizeRData(rrsetType, rdata string) string {
	rrsetType = strings.ToUpper(rrsetType)
	if rrsetType == "TXT" && len(rdata) >= 2 && strings.HasPrefix(rdata, "\"") && strings.HasSuffix(rdata, "\"") {
		return strings.TrimSuffix(strings.TrimPrefix(rdata, "\""), "\"")
	}

	if !fqdnRDataTypes[rrsetType] {
		return rdata
	}
	idx := strings.LastIndexAny(rdata, " \t")
	if idx < 0 {
		return strings.TrimSuffix(rdata, ".")
	}
	prefix := rdata[:idx+1]
	host := rdata[idx+1:]
	return prefix + strings.TrimSuffix(host, ".")
}

// timePtrToValue converts a *time.Time (typical of SDK response types) into a
// types.String containing an RFC3339 representation, mapping nil to
// types.StringNull().
func timePtrToValue(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}

func intPtrToValue(i *int) types.Int64 {
	if i == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*i))
}

func intToStringValue(i int) types.String {
	return types.StringValue(strconv.Itoa(i))
}

func stringListValueToStrings(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	var out []string
	if list.IsNull() || list.IsUnknown() {
		return out, nil
	}
	diags := list.ElementsAs(ctx, &out, false)
	return out, diags
}

func parseOptionalRFC3339(value types.String, name string) (*time.Time, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}
	parsed, err := time.Parse(time.RFC3339, value.ValueString())
	if err != nil {
		diags.AddError(
			"Invalid RFC3339 timestamp",
			fmt.Sprintf("%s must be an RFC3339 timestamp such as 2026-05-31T12:00:00Z: %s", name, err),
		)
		return nil, diags
	}
	return &parsed, diags
}

var tagEnrichedAttrTypes = map[string]attr.Type{
	"tag_id": types.StringType,
	"label":  types.StringType,
	"color":  types.StringType,
}

func tagEnrichedListValue(tags []models.TagEnriched) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	objType := types.ObjectType{AttrTypes: tagEnrichedAttrTypes}
	values := make([]attr.Value, len(tags))
	for i, t := range tags {
		obj, d := types.ObjectValue(tagEnrichedAttrTypes, map[string]attr.Value{
			"tag_id": types.StringValue(string(t.TagID)),
			"label":  types.StringValue(t.Label),
			"color":  types.StringValue(string(t.Color)),
		})
		diags.Append(d...)
		if diags.HasError() {
			return types.ListNull(objType), diags
		}
		values[i] = obj
	}
	list, d := types.ListValue(objType, values)
	diags.Append(d...)
	return list, diags
}

// formatAPIError returns a multi-line string describing err with as much
// detail as the SDK exposes. For *opusdns.APIError values it surfaces the
// status code, error code, message, request id, the raw response body, and
// any structured `details` map. For other errors it returns err.Error().
func formatAPIError(err error) string {
	var apiErr *opusdns.APIError
	if !errors.As(err, &apiErr) {
		return err.Error()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s", apiErr.Error())
	if apiErr.RequestID != "" {
		fmt.Fprintf(&b, "\nrequest_id: %s", apiErr.RequestID)
	}
	if len(apiErr.Details) > 0 {
		if data, mErr := json.MarshalIndent(apiErr.Details, "", "  "); mErr == nil {
			fmt.Fprintf(&b, "\ndetails:\n%s", data)
		} else {
			fmt.Fprintf(&b, "\ndetails: %+v", apiErr.Details)
		}
	}
	if apiErr.RawBody != "" {
		fmt.Fprintf(&b, "\nresponse body:\n%s", strings.TrimSpace(apiErr.RawBody))
	}
	return b.String()
}
