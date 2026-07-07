resource "truenas_user" "deploy" {
  username  = "deploy"
  full_name = "Deploy User"
  password  = "changeme"
  shell     = "/bin/bash"
}
