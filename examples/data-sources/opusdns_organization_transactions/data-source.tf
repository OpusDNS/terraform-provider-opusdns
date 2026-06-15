# Last 25 succeeded domain transactions for the caller's organization.
data "opusdns_organization_transactions" "succeeded_domains" {
  me           = true
  page_size    = 25
  sort_by      = "created_on"
  sort_order   = "desc"
  product_type = "domain"
  status       = "succeeded"
}

output "succeeded_domain_amounts" {
  value = [
    for t in data.opusdns_organization_transactions.succeeded_domains.transactions :
    "${t.product_reference} ${t.action}: ${t.amount} ${t.currency}"
  ]
}
