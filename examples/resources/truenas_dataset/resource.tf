# A plain dataset under an existing pool.
resource "truenas_dataset" "data" {
  name        = "tank/data"
  compression = "lz4"
  comments    = "managed by terraform"
}

# Renaming: changing `name` renames the dataset in place (data, child datasets
# and snapshots move with it; `guid` is unchanged). TrueNAS performs NO safety
# checks on a rename: shares, iSCSI extents, NVMe-oF namespaces, snapshot /
# replication / rsync / cloud sync tasks and VM disks that point at the old
# path are not updated and break until re-pointed. The provider refuses the
# rename while any of them references the dataset; allow_rename_in_use = true
# overrides that. Resources that reference `mountpoint`, like the share below,
# plan their own update in the same apply, so the share is briefly broken.
resource "truenas_dataset" "projects" {
  name = "tank/projects" # was "tank/project": applied as a rename
}

resource "truenas_smb_share" "projects" {
  name = "projects"
  path = truenas_dataset.projects.mountpoint
}

# A passphrase-encrypted dataset. The encryption inputs are create-only:
# changing any of them recreates the dataset. encryption_passphrase is
# write-only — it is used at create time and never stored in state, so supply
# it from a sensitive variable.
variable "dataset_passphrase" {
  type      = string
  sensitive = true
}

resource "truenas_dataset" "secret" {
  name                  = "tank/secret"
  encryption            = true
  inherit_encryption    = false
  encryption_algorithm  = "AES-256-GCM" # ignored on TrueNAS 27.0+ (fixed server-side)
  encryption_passphrase = var.dataset_passphrase

  # Read back after apply:
  #   encrypted  = true
  #   key_format = "PASSPHRASE"
  #   locked     = false
}

# A key-encrypted dataset with a TrueNAS-generated key (key_format = "HEX").
resource "truenas_dataset" "keyed" {
  name                    = "tank/keyed"
  encryption              = true
  inherit_encryption      = false
  encryption_generate_key = true
}

# Cloning a dataset from a snapshot: the clone starts with the snapshot's
# contents and shares its blocks until it diverges. `origin` is create-only
# (changing it replaces the dataset); reference the snapshot resource so the
# clone is destroyed before the snapshot, which ZFS cannot remove while clones
# of it exist.
resource "truenas_snapshot" "template" {
  dataset = truenas_dataset.data.name
  name    = "template"
}

resource "truenas_dataset" "from_template" {
  name   = "tank/data-copy"
  origin = truenas_snapshot.template.id # "tank/data@template"
}
