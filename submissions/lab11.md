# Lab 11 — Reproducible Builds of QuickNotes with Nix

## Task 1 — Reproducible Go Build

### Implementation

I created a Nix flake to build the QuickNotes Go application in a reproducible environment.

The flake uses `nixpkgs` from the `nixos-25.11` branch, with the exact revision pinned in `flake.lock`.

It supports two Linux architectures:
- `aarch64-linux`
- `x86_64-linux`

The application is built using `pkgs.buildGoModule` with the following configuration:

- Go version: 1.25.10 (Go >= 1.24)
- `CGO_ENABLED=0`
- `vendorHash = null`
- `ldflags = [ "-s" "-w" ]`
- Go tests enabled with `doCheck = true`

Since QuickNotes does not use external Go modules, `vendorHash = null` is sufficient.

The flake exposes two package outputs:
- `packages.<system>.quicknotes`
- `packages.<system>.default`

### Build Commands

```bash
nix build .#quicknotes
nix build .#default
```

### Reproducibility Verification

I built the application twice in separate Nix containers with independent Nix stores.

For each build, I calculated the Nix hash of the resulting binary:

```bash
nix hash path ./result/bin/quicknotes
```

Results:

| Build | Binary hash |
|---|---|
| Build 1 | `sha256-+W1UAE9JRaA0gH5aFDVNuexXjIj4xBpxYBUuAba9AUA=` |
| Build 2 | `sha256-+W1UAE9JRaA0gH5aFDVNuexXjIj4xBpxYBUuAba9AUA=` |

**Result:** Both builds produced identical binary hashes.

### Application Verification

I started the compiled QuickNotes application and checked its health endpoint.

Command:

```bash
curl -i http://localhost:8080/health
```

Response:

```http
HTTP/1.1 200 OK
Content-Type: application/json

{"notes":0,"status":"ok"}
```

The application started successfully and returned HTTP 200.

### Development Shell

I configured a Nix development shell containing:
- Go
- gopls
- golangci-lint

Verification command:

```bash
nix develop -c sh -c 'go version; gopls version; golangci-lint version'
```

Results:

| Tool | Version |
|---|---|
| Go | 1.25.10 |
| gopls | 0.20.0 |
| golangci-lint | 2.6.2 |

All required development tools were available.

### Discussion

**a. Why is `flake.lock` important?**

`flake.lock` records the exact revisions and hashes of the flake inputs. This prevents unexpected dependency changes and allows different machines to use the same dependency versions.

**b. How does Nix improve build reproducibility?**

Nix provides a declarative build environment with pinned dependencies. It reduces differences caused by system packages, toolchain versions, and host configurations.

**c. Why is `CGO_ENABLED=0` used?**

Disabling CGO removes the dependency on the system C toolchain and C libraries. This makes the Go binary easier to distribute and run in minimal containers.

**d. What is the purpose of a Nix development shell?**

A Nix development shell provides a consistent set of development tools without requiring manual installation. Developers can use the same toolchain versions across different machines.

---

## Task 2 — Reproducible Docker Image

### Implementation

I extended the Nix flake with a Docker image package using `pkgs.dockerTools.buildImage`.

The Docker image has the following configuration:

- Image name: `quicknotes-nix`
- Image tag: `latest`
- Entrypoint: QuickNotes executable
- Exposed port: `8080/tcp`
- User: `10001:10001`
- Working directory: `/app`

The image includes a writable `/app` directory so that QuickNotes can create its application data directory while running as a non-root user.

The image is built directly by Nix without requiring a Docker daemon during the build.

### Build Command

```bash
nix build .#docker
```

### Reproducibility Verification

I performed two independent builds using separate Nix containers and independent Nix stores.

For each build, I calculated the SHA-256 of the generated Docker archive:

```bash
sha256sum "$(readlink -f result)"
```

Results:

| Build | Docker archive SHA-256 |
|---|---|
| Build 1 | `0131465c839426c98737a058846a647c6b1786c360d7b8bea9276e3ed30a0d2f` |
| Build 2 | `0131465c839426c98737a058846a647c6b1786c360d7b8bea9276e3ed30a0d2f` |

**Result:** Both builds produced byte-for-byte identical Docker archives.

### Runtime Verification

I loaded the generated image into Docker and started the container.

Commands:

```bash
docker load -i quicknotes-nix.tar.gz

docker run --rm -d \
  --name quicknotes-nix-test \
  -p 8080:8080 \
  quicknotes-nix:latest

curl -i http://localhost:8080/health
```

Response:

```http
HTTP/1.1 200 OK
Content-Type: application/json

{"notes":0,"status":"ok"}
```

The container started successfully as a non-root user, and the application returned HTTP 200.

### Comparison with a Conventional Dockerfile

I created a multi-stage Dockerfile for comparison.

The conventional Docker image uses:
- `golang:1.24-alpine` for building the application.
- `alpine:3.21` as the runtime image.
- `CGO_ENABLED=0` and `-ldflags="-s -w"`.
- A non-root application user.

I built the Docker image twice without using the build cache:

```bash
docker build --no-cache -t qn-lab6:run1 ./app
docker build --no-cache -t qn-lab6:run2 ./app
```

I then compared the image IDs and sizes:

```bash
docker image inspect qn-lab6:run1 qn-lab6:run2 \
  --format '{{.RepoTags}} | ID={{.Id}} | Size={{.Size}} bytes'
```

Results:

| Image | Image ID | Size |
|---|---|---|
| Nix | `sha256:bbab59d3d515906981cec0a3e13d6be6fb77a740d610d1d7b63b3a293a877da4` | 9,186,278 bytes |
| Dockerfile run 1 | `sha256:743014e28ea56f3b885134fbb2a61e00f4f5cb4748f6a99619df436af54b0c27` | 6,274,974 bytes |
| Dockerfile run 2 | `sha256:f4f99e576ab59a4fbc6b1594f770f01003ad69308fed65b2124dfefda9c48041` | 6,274,975 bytes |

The two Dockerfile builds produced different image IDs and differed in size by one byte.

The Nix image was larger, but its independently generated archives had identical SHA-256 hashes.

The comparison demonstrates that the tested Nix configuration produced reproducible image archives, while the conventional Dockerfile configuration did not produce identical image IDs.

### Discussion

**e. Why use `dockerTools.buildImage` instead of a Dockerfile?**

`dockerTools.buildImage` builds Docker images using Nix derivations. It allows dependencies and image contents to be managed declaratively and supports building images without a Docker daemon.

**f. Why did the Nix Docker builds produce identical hashes?**

Both builds used identical source code, pinned dependencies, and the same Nix build configuration. Nix generated deterministic image archives, resulting in matching SHA-256 hashes.

**g. Does this mean Dockerfile builds cannot be reproducible?**

No. Dockerfile builds can also be reproducible if base images, dependencies, timestamps, and build metadata are controlled. However, the two Dockerfile builds in this experiment produced different image IDs.

---

## Bonus — GitHub Actions Reproducibility Check

### Implementation

I created `.github/workflows/nix-repro.yml` to verify Docker image reproducibility automatically.

The workflow runs on pushes and pull requests.

It contains three jobs:

1. `build-1`: builds the Docker image using Nix and calculates its SHA-256.
2. `build-2`: independently builds the same image and calculates its SHA-256.
3. `compare`: downloads both hashes and verifies that they match.

The two build jobs run on separate GitHub-hosted runners.

The Nix installer action is pinned to the following full commit SHA:

```text
2cb76beab460be7e44442c7ebdcbc30b638416df
```

If the hashes differ, the comparison job exits with a non-zero status and the workflow fails.

### Red Run — Intentional Failure

**Workflow:** [Red Run](https://github.com/rslwqr/DevOps-Intro/actions/runs/38614520510)

**Commit:** `dc84645`

Results:

| Job | Status |
|---|---|
| build-1 | Success |
| build-2 | Success |
| compare | Failure |

To test the failure detection mechanism, I temporarily replaced the second hash with:

```bash
HASH2="intentionally-different-hash"
```

Both build jobs completed successfully, but the comparison job failed as expected.

This demonstrated that the workflow correctly detects a hash mismatch.

### Green Run — Successful Verification

**Workflow:** [Green Run](https://github.com/rslwqr/DevOps-Intro/actions/runs/38615253751)

**Commit:** `679c9fd`

Results:

| Job | Status |
|---|---|
| build-1 | Success |
| build-2 | Success |
| compare | Success |

I restored the comparison of the actual hashes:

```bash
HASH1=$(cat first/hash.txt)
HASH2=$(cat second/hash.txt)
```

Both independently generated hashes matched, and all three jobs completed successfully.

### Discussion

**h. Why are two independent GitHub Actions runners used?**

Independent runners help verify that the build result does not depend on a particular machine's local state or build cache.

**i. Why compare SHA-256 hashes?**

SHA-256 produces a fingerprint of the generated archive. Matching hashes provide strong evidence that both archives contain identical bytes.

**j. Why demonstrate both a red and a green run?**

The red run verifies that the workflow detects mismatches and fails correctly. The green run demonstrates that the independent builds produce matching hashes when the comparison uses their actual outputs.

---

## Conclusion

I implemented reproducible builds for the QuickNotes application using Nix flakes.

Two independent Go builds produced identical binary hashes, and the compiled application successfully passed the health check.

The Nix-generated Docker image also produced identical SHA-256 hashes across independent builds and ran successfully as a non-root user.

The conventional Dockerfile builds produced smaller images but different image IDs in the tested configuration.

Finally, GitHub Actions successfully verified reproducibility using two independent build jobs and a comparison job. Both intentional failure and successful verification were demonstrated.