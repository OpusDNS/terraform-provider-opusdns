resource "opusdns_contact" "registrant" {
  first_name  = "Jane"
  last_name   = "Doe"
  email       = "jane.doe@example.com"
  phone       = "+1.5555550100"
  street      = "123 Main St"
  city        = "Springfield"
  state       = "IL"
  postal_code = "62701"
  country     = "US"
  org         = "Example Corp"
  disclose    = false
}
