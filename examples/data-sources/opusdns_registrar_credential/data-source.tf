# Read metadata for a single registrar credential. The credentials
# payload itself is not returned by the API.
data "opusdns_registrar_credential" "primary" {
  registrar_credential_id = "registrar_credential_01jxxxxxxxxxxxxxxxxxxxxxxx"
}

output "primary_registrar" {
  value = data.opusdns_registrar_credential.primary.registrar
}
