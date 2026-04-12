resource "aws_s3_bucket" "photos" {
  bucket        = "${var.app_name}-photos-${data.aws_caller_identity.current.account_id}"
  force_destroy = true

  tags = { Name = "${var.app_name}-photos" }
}

resource "aws_s3_bucket_ownership_controls" "photos" {
  bucket = aws_s3_bucket.photos.id

  rule {
    object_ownership = "BucketOwnerPreferred"
  }
}

resource "aws_s3_bucket_public_access_block" "photos" {
  bucket                  = aws_s3_bucket.photos.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "photos" {
  bucket = aws_s3_bucket.photos.id
  versioning_configuration {
    status = "Enabled"
  }
}

data "aws_caller_identity" "current" {}
