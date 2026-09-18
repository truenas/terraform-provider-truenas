# Singleton resource: manages which app catalog trains are preferred when
# browsing and installing applications. "label" and "location" are
# read-only (catalog.update accepts only "preferred_trains"). Terraform
# destroy only removes this resource from state; the catalog configuration
# is left in place.
resource "truenas_catalog_config" "config" {
  preferred_trains = ["stable", "community"]
}
