# Fetch a single billing transaction by id.
data "opusdns_organization_transaction" "example" {
  me                     = true
  billing_transaction_id = "btx_01h45ytscbebyvny4gc8cr8ma2"
}

output "transaction_status" {
  value = "${data.opusdns_organization_transaction.example.status}: ${data.opusdns_organization_transaction.example.amount} ${data.opusdns_organization_transaction.example.currency}"
}
