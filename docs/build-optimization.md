# Build Time Optimization Analysis

## Current Build Performance

**Local build:** ~0.5 seconds (very fast, already optimized)
**Project size:** 75MB, 45 Go files

## Current Setup Analysis

### ✅ What's Already Good

1. **Dockerfile uses multi-stage build** - Build and runtime stages separated
2. **Go mod caching layer** - `go mod download` runs before copying source
3. **.dockerignore exists** - Prevents unnecessary files from bloating build context
4. **Minimal dependencies** - Only 2 direct dependencies (BurntSushi/toml, Emergent SDK)
5. **Fast local builds** - Go's incremental compilation works well

### ❌ Optimization Opportunities

#### 1. **GitHub Actions: No Go Module Caching**

**Current state:**
```yaml
- uses: actions/setup-go@v5
  with:
    go-version: '1.23'
- name: Build binary
  run: go build ...
```

**Problem:** Downloads all modules on every run (~10-15 seconds)

**Solution:** Add Go module caching
```yaml
- uses: actions/setup-go@v5
  with:
    go-version: '1.23'
    cache: true  # ← Add this
```

OR explicit caching:
```yaml
- uses: actions/cache@v4
  with:
    path: |
      ~/.cache/go-build
      ~/go/pkg/mod
    key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
    restore-keys: |
      ${{ runner.os }}-go-
```

**Impact:** 10-15 second savings per workflow run

---

#### 2. **Docker Build: No Build Cache Between Runs**

**Current Dockerfile:**
```dockerfile
COPY go.mod go.sum ./
RUN go mod download  # ← This is cached, but...
COPY . .             # ← This invalidates cache on any file change
RUN go build ...     # ← Rebuilds everything
```

**Problem:** Any source code change invalidates the build cache

**Solution A: Use Docker BuildKit cache mounts (recommended)**
```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./

# Cache Go modules across builds
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Cache Go build cache across builds
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-s -w -X main.Version=${VERSION}" \
    -o /specmcp ./cmd/specmcp/

FROM alpine:3.21
# ... rest unchanged
```

**Solution B: Multi-stage with separate compilation cache layer**
```dockerfile
FROM golang:1.25-alpine AS modules
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

FROM modules AS builder
RUN apk add --no-cache git ca-certificates
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-s -w -X main.Version=${VERSION}" \
    -o /specmcp ./cmd/specmcp/
```

**Impact:** 5-10 second savings on incremental Docker builds

**Enable in GitHub Actions:**
```yaml
- name: Set up Docker Buildx
  uses: docker/setup-buildx-action@v3
  
- name: Build Docker image
  uses: docker/build-push-action@v5
  with:
    context: .
    cache-from: type=gha
    cache-to: type=gha,mode=max
```

---

#### 3. **GitHub Actions: Duplicate Workflows**

**Problem:** We have BOTH `ci.yml` and `build-release.yml` that do similar work:
- Both build binaries
- Both run on push to main and tags
- Both test the code
- Wastes runner minutes

**Solution:** Consolidate into a single workflow with job dependencies

---

#### 4. **Arch Linux Build: Downloads pacman packages every time**

**Current:**
```yaml
- name: Install dependencies
  run: |
    pacman -Syu --noconfirm
    pacman -S --noconfirm base-devel git go
```

**Problem:** Downloads and installs packages on every run

**Solution:** Use a custom Docker image with pre-installed dependencies
```dockerfile
# .github/builders/archlinux.Dockerfile
FROM archlinux:latest
RUN pacman -Syu --noconfirm && \
    pacman -S --noconfirm base-devel git go && \
    pacman -Scc --noconfirm
RUN useradd -m builder
```

Build and push to GitHub Container Registry, then use:
```yaml
container:
  image: ghcr.io/emergent-company/specmcp-builder:latest
```

**Impact:** 30-60 second savings on Arch builds

---

#### 5. **Matrix Builds: Sequential Instead of Parallel**

**Current:** Matrix strategy is good, but artifacts are uploaded individually

**Optimization:** Already parallelized well, but could combine checksums generation:
```yaml
- name: Generate checksums
  run: |
    cd dist
    find . -name "*.tar.gz" -exec sha256sum {} \; > checksums.txt
```

---

#### 6. **Missing: Local Docker Build Cache**

**Problem:** Developers running `task docker` don't benefit from BuildKit

**Solution:** Update Taskfile.yml
```yaml
docker:
  desc: Build Docker image with cache
  cmds:
    - DOCKER_BUILDKIT=1 docker build --build-arg BUILDKIT_INLINE_CACHE=1 -t specmcp .

docker-push:
  desc: Push Docker image
  cmds:
    - docker push specmcp
```

---

## Implementation Priority

### High Priority (Quick Wins)

1. **Add Go module caching to GitHub Actions** (5 min, 10-15s savings per run)
2. **Consolidate duplicate workflows** (30 min, reduces complexity + saves minutes)
3. **Enable Docker BuildKit caching in GHA** (10 min, 5-10s savings per build)

### Medium Priority

4. **Update Dockerfile with BuildKit cache mounts** (15 min, better local dev experience)
5. **Enable DOCKER_BUILDKIT in Taskfile** (5 min, local speedup)

### Low Priority (Nice to Have)

6. **Create custom Arch builder image** (1 hour setup, 30-60s savings but only for Arch builds)

---

## Estimated Time Savings

**Per GitHub Actions workflow run:**
- Go module caching: **10-15 seconds**
- Docker BuildKit caching: **5-10 seconds**
- Arch package caching: **30-60 seconds**

**Total potential savings: 45-85 seconds per workflow run**

With ~10 pushes/day on active development: **7-14 minutes saved per day**

**Per year:** ~40-80 hours of CI time saved

---

## Additional Optimizations (Future)

### 1. **Parallel Testing**
```yaml
- name: Run tests
  run: go test -parallel 4 ./...
```

### 2. **Incremental Go Builds**
```yaml
- name: Build with cache
  run: |
    go build -trimpath \
      -buildvcs=false \
      -o dist/specmcp ./cmd/specmcp/
```

### 3. **Cross-compilation Cache**
Use `go install` to cache tools:
```yaml
- name: Cache tools
  uses: actions/cache@v4
  with:
    path: ~/go/bin
    key: ${{ runner.os }}-go-tools-${{ hashFiles('**/go.mod') }}
```

### 4. **Dependabot for Actions**
Keep actions up-to-date automatically:
```yaml
# .github/dependabot.yml
version: 2
updates:
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
```

---

## Recommended Implementation Order

1. ✅ Add Go module caching to workflows (immediate win)
2. ✅ Consolidate ci.yml and build-release.yml (reduce duplication)
3. ✅ Add Docker BuildKit caching to GHA (faster Docker builds)
4. ✅ Update Dockerfile with cache mounts (better local DX)
5. ✅ Update Taskfile with DOCKER_BUILDKIT (consistency)
6. ⏸️ Create custom Arch builder image (defer until build times become painful)

---

## Benchmarking Plan

After implementing optimizations, track metrics:

**Before:**
- [ ] Record baseline workflow times (check recent runs)
- [ ] Record local `task docker` time
- [ ] Record Arch package build time

**After:**
- [ ] Compare workflow times
- [ ] Measure actual savings
- [ ] Document in README

**Metrics to track:**
- Total workflow duration
- Individual job durations
- Cache hit rates
- GitHub Actions minutes usage
