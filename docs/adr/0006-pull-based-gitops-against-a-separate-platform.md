# Generated projects deploy through pull-based GitOps to a separate platform

Generated projects run on a local `kind` cluster, not in the cloud. A local cluster is free for as long as you need it, while new AWS accounts only get credits that expire. GitHub Actions can't reach a cluster on a laptop, so CI never deploys. It builds multi-arch images, pushes them to GHCR, and commits the new image tag. Argo CD running in the cluster sees the commit and deploys it. The cluster and its shared services (Argo CD, ingress, observability, CloudNativePG, Sealed Secrets) live in a separate `platform` repo, not in sow. sow creates repos while `platform` runs infrastructure, and the two change at different speeds. Generated projects rely only on the platform contract. How CI commits the image tag past a protected `main` is still open; see the roadmap's open questions for 0.8.

See `docs/related-projects/platform.md` for the contract.
