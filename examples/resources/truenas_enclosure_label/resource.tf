# Manages the label of one pre-existing enclosure. The enclosure's label as
# found immediately before this resource starts managing it is captured and
# restored automatically on `terraform destroy` -- see the resource's schema
# description for the full restore-on-destroy and import safety contract.
resource "truenas_enclosure_label" "head_unit" {
  id    = "3b0ad6d1c00006e0"
  label = "Head Unit"
}
