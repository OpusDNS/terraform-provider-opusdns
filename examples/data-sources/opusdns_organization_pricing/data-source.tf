# Pricing for .com domain registrations under the caller's organization.
data "opusdns_organization_pricing" "dot_com_create" {
  me             = true
  product_type   = "domain"
  product_action = "create"
  product_class  = "com"
}

output "dot_com_create_price" {
  value = try(
    "${data.opusdns_organization_pricing.dot_com_create.prices[0].price} ${data.opusdns_organization_pricing.dot_com_create.prices[0].currency}",
    null,
  )
}

# All actions and TLDs for the domain product type.
data "opusdns_organization_pricing" "all_domains" {
  me           = true
  product_type = "domain"
}
