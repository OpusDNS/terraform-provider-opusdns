# Releasing terraform-provider-opusdns

How to publish a new release to the [HCP Terraform Registry](https://registry.terraform.io/providers/opusdns/opusdns).

## Prerequisites (one-time setup)

### HCP Terraform Registry

1. Sign in to [registry.terraform.io](https://registry.terraform.io) with the GitHub account that owns the `OpusDNS` organization.
2. **Publish → Provider** → select `terraform-provider-opusdns`.
3. Upload the GPG public key under **Settings → Signing Keys**.

### GPG Key

```bash
# Generate (Ed25519 recommended)
gpg --full-generate-key
# → ECC / Curve 25519, name "OpusDNS Release Signing", email release@opusdns.com

# Export public key → upload to HCP Terraform Registry
gpg --armor --export "OpusDNS Release Signing" > opusdns-release.pub

# Export private key → store as GitHub Secret (base64-encoded)
gpg --armor --export-secret-keys "OpusDNS Release Signing" | base64 -w0
```

### GitHub Secrets

Add these in **Settings → Secrets and variables → Actions**:

| Secret | Description |
|--------|-------------|
| `GPG_PRIVATE_KEY` | Base64-encoded armored GPG private key |
| `GPG_PASSPHRASE` | GPG key passphrase |

### GitHub Environment: `preview1`

Create under **Settings → Environments → preview1**:

| Secret | Description |
|--------|-------------|
| `OPUSDNS_API_KEY` | API key for the preview1 test org |
| `OPUSDNS_API_ENDPOINT` | `https://api.preview1.opusdns.dev` |

## Release Process

### 1. Ensure CI is green

All checks run automatically on every PR and push to `main`:

- **CI** (`ci.yml`) — gofmt, go vet, golangci-lint, build, unit tests, `terraform fmt` on examples
- **Acceptance Tests** (`acceptance.yml`) — full test suite against preview1 (runs on push to `main` when `internal/` or `go.*` changes, or manually via workflow dispatch)

No need to run tests locally — the pipeline handles it.

### 2. Tag the release

```bash
git checkout main
git pull origin main

# Semantic version with v prefix
git tag -a v1.1.0 -m "Release v1.1.0"
git push origin v1.1.0
```

### 3. Automated release pipeline

Pushing a `v*` tag triggers `.github/workflows/release.yml`:

1. ✅ Runs acceptance tests against preview1
2. 🔨 Builds cross-platform binaries (linux/darwin/windows × amd64/arm64)
3. 🔐 Signs SHA256SUMS with GPG
4. 📦 Creates a GitHub Release with all artifacts
5. 🚀 HCP Terraform Registry detects the release via webhook automatically

If acceptance tests fail, the release is blocked.

### 4. Verify

1. Check [GitHub Releases](../../releases)
2. Verify on [registry.terraform.io/providers/opusdns/opusdns](https://registry.terraform.io/providers/opusdns/opusdns)
3. Quick smoke test:
   ```bash
   mkdir /tmp/test-release && cd /tmp/test-release
   cat > main.tf << 'EOF'
   terraform {
     required_providers {
       opusdns = {
         source  = "opusdns/opusdns"
         version = "1.1.0"  # ← the version you just released
       }
     }
   }
   provider "opusdns" {}
   EOF
   terraform init
   ```

## Versioning

Follow [Semantic Versioning](https://semver.org/):

| Bump | When | Example |
|------|------|---------|
| **MAJOR** | Breaking changes (removed/renamed attributes, changed behavior) | v2.0.0 |
| **MINOR** | New resources, data sources, or attributes (backward compatible) | v1.1.0 |
| **PATCH** | Bug fixes, documentation updates | v1.0.1 |

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Registry doesn't detect the release | Check webhook in Registry Settings; verify `SHA256SUMS` + `SHA256SUMS.sig` exist in the release |
| GPG signature fails | Ensure the public key on the Registry matches the private key in `GPG_PRIVATE_KEY` secret; check expiry with `gpg --list-keys` |
| GoReleaser fails | Run `go mod tidy` and commit; ensure no uncommitted changes; tag must be `vX.Y.Z` format |
| Acceptance tests fail on release | Fix the issue, delete the tag (`git push --delete origin v1.1.0`), re-tag after fix |

