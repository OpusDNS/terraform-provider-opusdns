package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
)

// dnssecRecordModel mirrors the schema in resource_domain_dnssec.go for use
// with ElementsAs / generic decoding from a types.List.
type dnssecRecordModel struct {
	ID         types.String `tfsdk:"id"`
	RecordType types.String `tfsdk:"record_type"`
	Algorithm  types.Int64  `tfsdk:"algorithm"`
	Digest     types.String `tfsdk:"digest"`
	DigestType types.Int64  `tfsdk:"digest_type"`
	Flags      types.Int64  `tfsdk:"flags"`
	KeyTag     types.Int64  `tfsdk:"key_tag"`
	Protocol   types.Int64  `tfsdk:"protocol"`
	PublicKey  types.String `tfsdk:"public_key"`
	CreatedOn  types.String `tfsdk:"created_on"`
	UpdatedOn  types.String `tfsdk:"updated_on"`
}

// dnssecRecordsFromList converts a `records` types.List (as defined by the
// resource's schema) into the SDK's slice of DomainDNSSECDataCreate.
//
// Null / unknown lists yield an empty slice (no records). Per-element nil
// strings/ints are forwarded as nil/zero pointers so the API can apply its own
// defaults.
func dnssecRecordsFromList(ctx context.Context, list types.List) ([]models.DomainDNSSECDataCreate, diag.Diagnostics) {
	var diags diag.Diagnostics

	if list.IsNull() || list.IsUnknown() || len(list.Elements()) == 0 {
		return nil, diags
	}

	var rows []dnssecRecordModel
	diags.Append(list.ElementsAs(ctx, &rows, false)...)
	if diags.HasError() {
		return nil, diags
	}

	out := make([]models.DomainDNSSECDataCreate, 0, len(rows))
	for _, r := range rows {
		entry := models.DomainDNSSECDataCreate{
			RecordType: models.DNSSECRecordType(r.RecordType.ValueString()),
			Algorithm:  models.DNSSECAlgorithm(r.Algorithm.ValueInt64()),
		}
		if !r.Digest.IsNull() && !r.Digest.IsUnknown() {
			s := r.Digest.ValueString()
			entry.Digest = &s
		}
		if !r.DigestType.IsNull() && !r.DigestType.IsUnknown() {
			dt := models.DNSSECDigestType(r.DigestType.ValueInt64())
			entry.DigestType = &dt
		}
		if !r.Flags.IsNull() && !r.Flags.IsUnknown() {
			f := int(r.Flags.ValueInt64())
			entry.Flags = &f
		}
		if !r.KeyTag.IsNull() && !r.KeyTag.IsUnknown() {
			k := int(r.KeyTag.ValueInt64())
			entry.KeyTag = &k
		}
		if !r.Protocol.IsNull() && !r.Protocol.IsUnknown() {
			p := int(r.Protocol.ValueInt64())
			entry.Protocol = &p
		}
		if !r.PublicKey.IsNull() && !r.PublicKey.IsUnknown() {
			p := r.PublicKey.ValueString()
			entry.PublicKey = &p
		}
		out = append(out, entry)
	}
	return out, diags
}

// dnssecRecordsToList converts a slice of DomainDNSSECDataResponse from the
// SDK into a types.List matching the resource schema.
func dnssecRecordsToList(ctx context.Context, in []models.DomainDNSSECDataResponse) (types.List, diag.Diagnostics) {
	values := make([]attr.Value, 0, len(in))
	for _, r := range in {
		obj, diags := types.ObjectValue(dnssecRecordObjectAttrTypes, map[string]attr.Value{
			"id":          types.StringValue(string(r.DomainDNSSECDataID)),
			"record_type": types.StringValue(string(r.RecordType)),
			"algorithm":   types.Int64Value(int64(r.Algorithm)),
			"digest":      stringPtrToValue(r.Digest),
			"digest_type": int64PtrToDigestTypeValue(r.DigestType),
			"flags":       int64PtrToValue(r.Flags),
			"key_tag":     int64PtrToValue(r.KeyTag),
			"protocol":    int64PtrToValue(r.Protocol),
			"public_key":  stringPtrToValue(r.PublicKey),
			"created_on":  timePtrToValue(r.CreatedOn),
			"updated_on":  timePtrToValue(r.UpdatedOn),
		})
		if diags.HasError() {
			return types.ListNull(dnssecRecordObjectType), diags
		}
		values = append(values, obj)
	}
	return types.ListValueMust(dnssecRecordObjectType, values), nil
}

// int64PtrToValue converts a *int (typical of SDK responses) to a types.Int64.
func int64PtrToValue(p *int) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*p))
}

// int64PtrToDigestTypeValue converts a *DNSSECDigestType to a types.Int64.
func int64PtrToDigestTypeValue(p *models.DNSSECDigestType) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*p))
}
