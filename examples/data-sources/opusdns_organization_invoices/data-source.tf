# Most recent invoices for the caller's organization.
data "opusdns_organization_invoices" "recent" {
  me        = true
  page_size = 10
}

output "outstanding_invoices" {
  value = [
    for i in data.opusdns_organization_invoices.recent.invoices :
    "${i.number} (${i.amount} ${i.currency}) due ${i.payment_due_date}"
    if !contains(["succeeded"], i.payment_status)
  ]
}
