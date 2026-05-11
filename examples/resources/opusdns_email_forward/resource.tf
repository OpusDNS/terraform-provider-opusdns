resource "opusdns_email_forward" "example" {
  hostname = "example.com"
  enabled  = true

  aliases = [
    {
      alias      = "info"
      forward_to = ["admin@mycompany.com"]
    },
    {
      alias      = "*"
      forward_to = ["catchall@mycompany.com"]
    },
  ]
}
