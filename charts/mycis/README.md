# mycis Helm chart

Deploys the mycis web application (Controls Tracker) to Kubernetes. Postgres is **not** included; set `DATABASE_URL` to a database you run elsewhere (managed service, in-cluster Postgres, and so on).

## Prerequisites

- Kubernetes 1.24+
- Helm 3.8+ (for OCI installs from GHCR)
- A Secret (or chart-managed secret for non-production) with:
  - `DATABASE_URL` — Postgres connection string
  - `APP_SESSION_KEY` — at least 32 characters (see app validation in `internal/config`)

## Install from OCI (after a release)

Releases push the chart to GitHub Container Registry. Replace `OWNER` with your GitHub org or user (lowercase) and `X.Y.Z` with the chart version (same as the git tag without `v`).

```bash
helm install mycis oci://ghcr.io/OWNER/helm-charts/mycis --version X.Y.Z \
  --set image.repository=ghcr.io/owner/repo \
  --set image.tag=X.Y.Z \
  --set secret.existingSecret=mycis-app
```

The container image is published as `ghcr.io/OWNER/REPO:VERSION` and `:latest` (lowercase path). Point `image.repository` / `image.tag` at that image.

Authenticate Helm to GHCR if the chart or image is private:

```bash
echo "$GITHUB_TOKEN" | helm registry login ghcr.io -u USERNAME --password-stdin
```

## Install from a local checkout

### Production-style (existing Secret)

Create a secret:

```bash
kubectl create secret generic mycis-app \
  --from-literal=DATABASE_URL='postgres://...' \
  --from-literal=APP_SESSION_KEY='your-32-plus-char-secret-here'
```

Install:

```bash
helm install mycis ./charts/mycis \
  --set image.repository=ghcr.io/myorg/mycis \
  --set image.tag=1.0.0 \
  --set secret.existingSecret=mycis-app \
  --set appCookieSecure=true
```

Set `appCookieSecure=true` when users reach the app over HTTPS (for example, TLS on Ingress).

### Development-only (chart creates the Secret)

Avoid putting real credentials in `values.yaml` files that are committed. For local clusters only:

```bash
helm install mycis ./charts/mycis \
  --set secret.create=true \
  --set secret.databaseUrl='postgres://...' \
  --set secret.sessionKey='0123456789abcdef0123456789abcdef' \
  --set image.repository=ghcr.io/myorg/mycis \
  --set image.tag=1.0.0
```

## Values reference

| Area | Notes |
|------|--------|
| `image.repository` / `image.tag` | Container image; if `tag` is empty, `Chart.AppVersion` is used |
| `secret.existingSecret` | Name of Secret with `DATABASE_URL` and `APP_SESSION_KEY` (recommended) |
| `secret.create` | If `true`, chart creates a Secret from `secret.databaseUrl` and `secret.sessionKey` |
| `ingress.enabled` | Optional HTTP(S) ingress |
| `appCookieSecure` | Session cookie `Secure` flag; use `true` behind HTTPS |
| `extraEnv` | Extra container env vars (e.g. `APP_NAME`) |

See [values.yaml](./values.yaml) for the full list.

## CI and releases

- **CI:** `helm lint` and `helm template` run in [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml).
- **Release:** Pushing a tag `v*` runs [`.github/workflows/release.yml`](../../.github/workflows/release.yml), which builds and pushes the Docker image to GHCR and pushes this chart to `oci://ghcr.io/<owner>/helm-charts`.

You can also run the release workflow manually and pass an existing tag.
