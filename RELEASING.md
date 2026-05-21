# Releasing terraform-provider-opusdns

This document describes how to prepare and publish a new release of the
OpusDNS Terraform Provider to the [Terraform Registry](https://registry.terraform.io).

## Prerequisites

### 1. Terraform Registry Setup (one-time)

1. Sign in to [registry.terraform.io](https://registry.terraform.io) with the
   GitHub account that owns the `OpusDNS` organization.
2. Navigate to **Publish → Provider**.
3. Select the `terraform-provider-opusdns` repository.
4. The registry namespace will be `opusdns`, provider name `opusdns`.
5. Add the GPG public key (see below) under **Settings → Signing Keys**.

### 2. GPG Key Setup (one-time)

The Terraform Registry requires that release artifacts are signed with a GPG key.

```bash
# Generate a new GPG key (Ed25519 recommended)
gpg --full-generate-key
# Choose: (9) ECC (sign and encrypt), Curve 25519
# Name: OpusDNS Release Signing
# Email: release@opusdns.com (or team email)
# Passphrase: <strong passphrase>

# Export the public key (upload this to Terraform Registry)
gpg --armor --export "OpusDNS Release Signing" > opusdns-release.pub

# Export the private key (store as GitHub Secret)
gpg --armor --export-secret-keys "OpusDNS Release Signing" | base64 -w0
# → Store the output as GitHub Secret: GPG_PRIVATE_KEY
```

### 3. GitHub Secrets Configuration

Add the following secrets to the repository (Settings → Secrets and variables → Actions):

| Secret | Description |
|---|---|
| `GPG_PRIVATE_KEY` | Base64-encoded GPG private key (armored export, then base64) |
| `GPG_PASSPHRASE` | Passphrase for the GPG key |

### 4. GitHub Environment: `preview1`

Create a GitHub Environment called `preview1` (Settings → Environments):

| Secret | Description |
|---|---|
| `OPUSDNS_API_KEY` | API key for the preview1 test organization |
| `OPUSDNS_API_ENDPOINT` | `https://api.preview1.opusdns.dev` |

Optional protection rules:
- Require approval for deployments (prevents accidental test runs)
- Restrict to `main` branch

## Release Process

### 1. Ensure tests pass

```bash
# Run locally (requires preview1 credentials)
export TF_ACC=1
export OPUSDNS_API_KEY="..."
export OPUSDNS_API_ENDPOINT="https://api.preview1.opusdns.dev"
go test ./internal/provider/ -v -count=1 -parallel=4 -timeout 30m
```

Or trigger the acceptance tests via GitHub Actions:
- Go to Actions → "Acceptance Tests" → Run workflow

### 2. Create a release tag

```bash
# Ensure you're on main with all changes merged
git checkout main
git pull origin main

# Tag the release (semantic versioning with v prefix)
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

### 3. Automated release

Pushing a `v*` tag triggers the release workflow which:
1. Runs the preview1 acceptance tests
2. Builds cross-platform binaries (linux/darwin/windows × amd64/arm64)
3. Signs the SHA256SUMS file with the GPG key
4. Creates a GitHub Release with all artifacts
5. The Terraform Registry automatically detects the new release via webhook

If the preview1 acceptance tests fail, the release job is blocked and no release artifacts are published.

### 4. Verify

- Check the [GitHub Releases](../../releases) page for the new release
- Verify on [registry.terraform.io/providers/opusdns/opusdns](https://registry.terraform.io/providers/opusdns/opusdns)
- Test installation:
  ```hcl
  terraform {
    required_providers {
      opusdns = {
        source  = "opusdns/opusdns"
        version = "~> 1.0"
      }
    }
  }
  ```

## Version Guidelines

Follow [Semantic Versioning](https://semver.org/):

- **MAJOR** (v2.0.0): Breaking changes (removed attributes, changed behavior). Removing provider auth modes such as `client_secret`, `org_id`, `username`, or `password` belongs here.
- **MINOR** (v1.1.0): New resources, data sources, or attributes (backward compatible)
- **PATCH** (v1.0.1): Bug fixes, documentation updates

## Troubleshooting

### Registry doesn't detect the release
- Ensure the GitHub webhook is configured (Registry Settings → Webhooks)
- Verify the release has the required `SHA256SUMS` and `SHA256SUMS.sig` files
- Check that the binary naming follows `terraform-provider-opusdns_<version>_<os>_<arch>.zip`

### GPG signature verification fails
- Ensure the public key uploaded to the Registry matches the private key used for signing
- Verify the key hasn't expired: `gpg --list-keys "OpusDNS Release Signing"`

### GoReleaser fails
- Check that `go mod tidy` produces no changes
- Ensure no uncommitted changes in the working tree
- Verify the tag matches semantic versioning: `v1.2.3` (not `1.2.3` or `v1.2.3-beta`)
