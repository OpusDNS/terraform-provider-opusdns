# Bug: NS rrset at delegated subdomain silently dropped after PATCH /v1/dns/{zone}/rrsets

## Summary

`PATCH /v1/dns/{zone_name}/rrsets` with an UPSERT op for an `NS` rrset at a delegated subdomain (e.g. `delegated.example.com`) returns `204 No Content`, but the rrset is **silently absent** from the subsequent `GET /v1/dns/{zone_name}` response. Only the apex `NS` and `SOA` are returned. No error or warning is surfaced to the client.

This breaks delegation use cases (e.g. delegating a sub-zone to another DNS provider) and surfaces as a refresh-plan drift in the OpusDNS Terraform provider — every `terraform apply` for a subdomain `NS` record re-plans `+ create` because the resource appears missing on the next refresh.

## Severity

High for any consumer that needs to delegate sub-zones (registrars, multi-provider topologies, CNAME-flattening services). Currently no error or response payload alerts the caller; the data is silently lost. This is also a Terraform-provider acceptance-test blocker.

## Environment

- API: live `https://api.opusdns.local` (commit corresponding to terraform-provider-opusdns branch `audit-api-changes-20260531` baseline, ~ fa71101)
- terraform-provider-opusdns: branch `audit-api-changes-20260531` (latest commit at time of report)
- Reproducer: `go test ./internal/provider -run TestAccRecordResource_NS -v -count=1`

## Reproduction

### Terraform configuration (acceptance test fixture)

```hcl
resource "opusdns_zone" "test" {
  name = "tfacc-<random>.com"
}

resource "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = "delegated"
  type      = "NS"
  records   = ["ns1.example.com", "ns2.example.com"]
}
```

### HTTP-level reproduction

1. Create the zone (returns 201 with `DnsChangesResponse` payload):

   ```
   POST /v1/dns/
   Content-Type: application/json

   { "name": "tfacc-<random>.com" }
   ```

2. Upsert the delegated NS rrset (returns 204):

   ```
   PATCH /v1/dns/tfacc-<random>.com/rrsets
   Content-Type: application/json

   {
     "ops": [
       {
         "op": "upsert",
         "rrset": {
           "name": "delegated",
           "type": "NS",
           "ttl": 60,
           "records": [
             { "rdata": "ns1.example.com" },
             { "rdata": "ns2.example.com" }
           ]
         }
       }
     ]
   }
   ```

3. Fetch the zone:

   ```
   GET /v1/dns/tfacc-<random>.com
   ```

   Response (relevant excerpt):

   ```json
   {
     "name": "tfacc-<random>.com",
     "rrsets": [
       { "name": "tfacc-<random>.com.", "type": "NS",  "records": [ ... apex defaults ... ] },
       { "name": "tfacc-<random>.com.", "type": "SOA", "records": [ ... ] }
     ]
   }
   ```

   The expected `{ name: "delegated.tfacc-<random>.com.", type: "NS", records: ["ns1.example.com", "ns2.example.com"] }` rrset is **missing**.

### Provider log evidence

`TF_LOG=DEBUG TF_ACC=1 go test ./internal/provider -run TestAccRecordResource_NS -v -count=1` produces:

```
[WARN] opusdns: record Read: rrset not found in zone:
  zone_name=tfacc-3303332254413021342.com
  lookup_name=delegated
  lookup_type=NS
  rrsets_count=2
  rrsets=[
    {name: "tfacc-3303332254413021342.com.", type: NS},
    {name: "tfacc-3303332254413021342.com.", type: SOA},
  ]
```

followed by the test failure:

```
After applying this test step, the refresh plan was not empty.
  # opusdns_record.test will be created
  + resource "opusdns_record" "test" {
      + name      = "delegated"
      + type      = "NS"
      + records   = ["ns1.example.com", "ns2.example.com"]
      ...
    }
Plan: 1 to add, 0 to change, 0 to destroy.
```

## Expected behaviour

After the PATCH succeeds, `GET /v1/dns/{zone_name}` should include the delegated NS rrset, e.g.:

```json
{
  "name": "delegated.tfacc-<random>.com.",
  "type": "NS",
  "ttl": 60,
  "records": [
    { "rdata": "ns1.example.com" },
    { "rdata": "ns2.example.com" }
  ]
}
```

Alternatively, if delegation rrsets are intentionally unsupported, the PATCH must return a 4xx (e.g. `422 Unprocessable Entity`) with a clear `DnsError` payload instead of silently discarding the data.

## Actual behaviour

PATCH returns `204`. GET returns the zone with only the apex `NS` and `SOA` rrsets. The delegated NS data is lost without any client-visible diagnostic.

## Suspected location

API service layer — `common/services/dns/dns.py:patch_rrsets` and `update_rrsets`. Key inspection points:

- `common/services/dns/dns.py:380` `patch_rrsets` — keys the upsert as `delegated.{zone}.:NS`, merges into `rrset_dict` (looks correct).
- `common/services/dns/dns.py:254` `update_rrsets` — calls `convert_rrset_names` then `power_dns_client.update_rrsets(changeset_baseline, zone, vanity_override)`.
- The drop most likely happens in `power_dns_client.update_rrsets`, in any apex/NS validation, or in PowerDNS itself rejecting/coalescing the delegation NS without surfacing an error back through the client.

A second possibility worth eliminating: the response model `DnsZoneResponse.rrsets` enricher filtering by `mutability` or `protected_reason` such that user-managed delegation NS gets stripped.

## Workarounds

None for the Terraform provider. Until fixed:

- Consumers cannot manage delegated NS records via `opusdns_record`.
- The provider acceptance test `TestAccRecordResource_NS` cannot pass.

## Related

- terraform-provider-opusdns: branch `audit-api-changes-20260531`
- Acceptance test: `internal/provider/resource_record_test.go:163` (`TestAccRecordResource_NS`)
- Test config helper: `internal/provider/resource_record_test.go:321` (`testAccRecordResourceConfig_NS`)
- Provider record refresh: `internal/provider/resource_record.go:135` (`RecordResource.Read`)
- API integration commit on provider side: `fa71101` ("Integrate recent changes to API for compatibility")

## Suggested acceptance criteria

1. `PATCH /v1/dns/{zone}/rrsets` with `{op:upsert, rrset:{name:"delegated", type:"NS", records:[...]}}` persists the rrset; subsequent `GET /v1/dns/{zone}` returns it under `name: "delegated.{zone}."`.
2. `terraform-provider-opusdns` acceptance test `TestAccRecordResource_NS` passes against the live API with no further provider changes.
3. If delegation NS rrsets are deliberately unsupported, document that explicitly and return a 4xx with a structured `DnsError` payload from the PATCH endpoint.
