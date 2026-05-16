# Manage a third-party registrar credential. The required keys in
# `credentials` depend on the chosen `registrar`. Values are sensitive
# and write-only at the API level — the provider cannot detect drift
# against the registrar after the initial write.
resource "opusdns_registrar_credential" "internetx_primary" {
  name      = "InternetX (primary)"
  registrar = "INTERNETX"

  credentials = {
    api_key  = "REPLACE_ME"
    username = "user@example.com"
  }
}
