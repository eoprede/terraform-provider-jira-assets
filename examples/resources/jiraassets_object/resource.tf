resource "jiraassets_object" "example_object" {
  type = "Host"
  attributes = {
    "ansible_managed"                = "false"
    "external_ips"              = "1.2.3.4"
    "terraform_managed"         = "true"
    "Status"                    = "Enabled"
  }
}
