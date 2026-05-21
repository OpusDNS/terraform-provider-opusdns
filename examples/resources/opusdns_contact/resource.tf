resource "opusdns_contact" "admin" {
  first_name  = "Jane"
  last_name   = "Doe"
  email       = "jane.doe@example.com"
  phone       = "+12125550199"
  street      = "123 Main St"
  city        = "New York"
  state       = "NY"
  postal_code = "10001"
  country     = "US"
  disclose    = false
}
