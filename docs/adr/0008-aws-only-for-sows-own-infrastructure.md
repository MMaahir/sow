# AWS is used only for sow's own infrastructure

The author wants AWS experience at no cost, but generated projects are hosted locally. AWS therefore appears only in sow's own infrastructure: the docs site on S3 and CloudFront. All of it is managed with Terraform and deployed from GitHub Actions through OIDC, with no stored access keys. A bootstrap config, applied once from a laptop, creates the state bucket, the OIDC provider, two roles and a $1 budget alert. Generated projects contain no AWS. Terraform was chosen over OpenTofu because it appears in more job listings, and the two use the same configuration language.

## Consequences

- A read-only plan role trusts `pull_request` runs, and an apply role trusts only `main` and tags. PR plans run with `-lock=false`, because S3 native locking would otherwise need write access to the state bucket. Applies run on `main` in a concurrency group.
- The budget is an alert, not a cap. AWS bills and notifies with a delay, so `infra/` documents a `terraform destroy` teardown.
