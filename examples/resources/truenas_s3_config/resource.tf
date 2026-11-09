# The S3 service configuration (TrueNAS 27.0 and later). One per system:
# this resource is never created or deleted on TrueNAS, only updated.
resource "truenas_dataset" "s3" {
  name = "tank/s3"
}

resource "truenas_s3_config" "s3" {
  listeners = [
    { address = "0.0.0.0", port = 9000 }, # plaintext on every address
  ]
  region    = "us-east-1"
  log_level = "NOTICE"

  # Buckets created through the S3 protocol (CreateBucket, by a key with
  # manage_buckets = true) get a dataset under this one. There is no
  # TrueNAS API for buckets.
  managed_root_dataset = truenas_dataset.s3.name

  default_audit = ["PutObject", "DeleteObject", "CreateBucket", "DeleteBucket"]
}

# Turn the service on.
resource "truenas_service" "s3" {
  service = "s3"
  enable  = true
  running = true
}
