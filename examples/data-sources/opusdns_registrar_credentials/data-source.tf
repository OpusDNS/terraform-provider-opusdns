# List all registrar credentials in the organization.
data "opusdns_registrar_credentials" "all" {}

# Filter by registrar.
data "opusdns_registrar_credentials" "internetx" {
  registrar = "INTERNETX"
}

output "internetx_names" {
  value = [for c in data.opusdns_registrar_credentials.internetx.registrar_credentials : c.name]
}
