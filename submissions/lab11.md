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

**a. Why might `go build` produce different outputs on two machines, even from the same Git commit?**

The builds may use different Go versions, dependency versions, build flags, paths, or environment settings. Build metadata, timestamps, and Go build IDs can also affect the output. The same Git commit does not guarantee an identical build environment.

**b. What exactly does `vendorHash` represent? What happens if `vendorHash = null`?**

`vendorHash` is the Nix hash of the dependency tree produced during the Go module vendoring step. It allows Nix to verify that the downloaded dependencies match the expected contents.

Setting `vendorHash = null` skips the dependency vendoring step. This is appropriate for QuickNotes because it uses only the Go standard library and has no external Go module dependencies.

**c. Why is `flake.lock` essential for reproducibility? What happens if it is deleted before the second build?**

`flake.lock` records the exact revision and content hash of `nixpkgs`. This ensures that independent builds use the same pinned dependencies and toolchain.

If it is deleted, Nix must resolve the input again. The channel may have advanced to a newer revision, potentially changing the build environment and the resulting hashes.

**d. What is the difference between `buildGoModule` and `buildGoApplication`? Which is more suitable for QuickNotes?**

`buildGoModule` is the standard Go builder provided by nixpkgs. It supports Go modules and integrates with the Nix dependency and build system.

`buildGoApplication` is commonly provided by the external `buildGo.nix` tooling and uses a different approach to configuring Go builds and dependencies.

I chose `buildGoModule` because QuickNotes is a small Go module, has no external dependencies, and can be built directly using the pinned nixpkgs toolchain.

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

**e. Why can `docker build` produce non-identical images from the same Dockerfile and Git commit?**

Docker builds may include timestamps, generated layer metadata, changing base-image tags, or packages downloaded from repositories that are not pinned to exact versions.

Even when the application source is unchanged, these external inputs and metadata can make the resulting image digests differ.

In this experiment, two Dockerfile builds produced different image IDs, while the independently generated Nix image archives had identical SHA-256 hashes.

**f. What can a security auditor verify with a reproducible image that cannot be established by a signature alone?**

A signature confirms that an image was signed by a particular identity, but it does not independently prove that the image was built from the claimed source code.

With a reproducible build, an auditor can independently rebuild the image from the reviewed source and pinned dependencies, then compare its digest with the distributed artifact.

A matching digest provides strong evidence that the distributed image corresponds to the independently verified build.

**g. What are the trade-offs of Nix reproducibility? Why is `docker build` still widely used?**

Nix provides stronger control over dependencies and build environments, but it introduces a learning curve, its own configuration language, and additional tooling.

Dockerfiles are more familiar to most development teams and integrate easily with existing CI/CD platforms, registries, and container workflows.

For many teams, Docker offers a simpler development experience, while Nix is useful when strict reproducibility and dependency control are priorities.

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

**h. Why is reproducibility verified in CI more valuable to a security auditor than reproducibility demonstrated only on a developer's laptop?**

A local reproducibility test may depend on the developer's machine, installed tools, or cached dependencies.

CI performs the verification in a documented and independently provisioned environment. It also creates logs and results that other people can inspect.

This makes the reproducibility claim easier to verify and provides evidence that the check runs automatically as part of the development process.

**i. Why use two parallel jobs instead of building twice in one job?**

Two parallel jobs run on separate fresh runners with independent environments and caches.

Running two builds in a single job could reuse local artifacts, caches, or other machine state. This could hide differences that would appear in genuinely independent builds.

Separate jobs provide stronger evidence that the result does not depend on the state of one runner.

**j. Where can timestamps affect a Nix build, and how does `dockerTools.buildImage` handle them?**

Timestamps can enter build outputs through generated files, archive metadata, compiler-generated metadata, or container image layers.

`SOURCE_DATE_EPOCH` is commonly used to provide a fixed timestamp for reproducible builds.

`dockerTools.buildImage` uses deterministic image metadata, including a fixed creation timestamp by default, rather than inserting the current build time.

This helps ensure that identical inputs produce identical image archives.

---

## Conclusion

I implemented reproducible builds for the QuickNotes application using Nix flakes.

Two independent Go builds produced identical binary hashes, and the compiled application successfully passed the health check.

The Nix-generated Docker image also produced identical SHA-256 hashes across independent builds and ran successfully as a non-root user.

The conventional Dockerfile builds produced smaller images but different image IDs in the tested configuration.

Finally, GitHub Actions successfully verified reproducibility using two independent build jobs and a comparison job. Both intentional failure and successful verification were demonstrated.