# Platform project: reference notes

Notes for the `platform` repo, gathered while planning sow. Read this before you start that project. Decided 2026-10-14.

## What it is

`platform` sets up a local Kubernetes cluster and the shared services every app relies on. You set it up once, and every project sow generates deploys into it. In a company, a platform team owns this layer and product teams build on it.

It's a separate project from sow. sow creates repos; `platform` runs infrastructure. They change for different reasons and at different speeds.

## What it runs

| Service | Job |
|---|---|
| `kind` cluster | The Kubernetes cluster, running locally on your Mac |
| Argo CD | Watches the app repos and deploys whatever changes in git (pull-based GitOps) |
| Ingress controller | Routes `<project>.localhost` to the right app |
| Grafana, Prometheus, Loki, Tempo | Store and display every app's metrics, logs and traces |
| OpenTelemetry Collector | Receives telemetry from apps and forwards it to the storage services above |
| CloudNativePG | Creates a Postgres database for each app that asks for one |
| Sealed Secrets | Lets apps commit encrypted secrets to git. Only the cluster can decrypt them |

## Why pull-based GitOps

GitHub Actions runs in the cloud and can't reach a cluster on your laptop, so CI never deploys. A change reaches the cluster like this:

1. You push to an app repo.
2. CI builds the images and pushes them to GHCR.
3. CI commits the new image tag to the repo's `deploy/` directory.
4. Argo CD, running in the cluster, sees the commit and deploys the new version.

## The contract with generated apps

Every app sow generates relies on these rules. Any of them can change, as long as `platform` and the sow `deploy` component change in the same release. How projects that were already generated pick up a change is an open question for sow 0.8.

| Rule | Value |
|---|---|
| Kubernetes namespace | One per project, with the same name as the project |
| Image names | `ghcr.io/<you>/<project>-<service>` |
| Deploy discovery | An Argo CD "app of apps" picks up each repo's `deploy/argocd/application.yaml` |
| URL | `<project>.localhost` |
| Telemetry | The platform sets `OTEL_EXPORTER_OTLP_ENDPOINT`, and apps send OTLP to it |
| Database | Apps request a database through a CloudNativePG resource and read credentials from a Secret named `<project>-db` |
| Secrets | Apps commit secrets only as `SealedSecret` resources, never as plain `Secret` manifests |
| Image architecture | Images are multi-arch (amd64 and arm64), so they run natively on an Apple Silicon Mac |
| Image tags | Images are tagged with the git commit SHA, never `latest` |
| Environments | One, called `local`. Charts keep per-environment values in `values-<env>.yaml` |

Each generated app repo contains its Helm chart in `deploy/helm/` and its Argo CD `Application` manifest in `deploy/argocd/`.

## Until this project exists

Generated projects run with Docker Compose only. The sow `deploy` component ships in sow 0.8, which needs `platform` running.
