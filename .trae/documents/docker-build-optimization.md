# Docker 构建优化计划

## 问题分析

当前 Docker 构建耗时 20-30 分钟，主要原因：

1. **`docker-build-notify.yml` 使用 QEMU 模拟构建 arm64**：在 amd64 runner 上通过 QEMU 模拟 arm64，速度极慢（比原生慢 5-10 倍）
2. **Dockerfile 未充分优化缓存**：`COPY . .` 会将所有源码复制到 Go 构建阶段，任何文件变更都导致 `go build` 缓存失效
3. **前端重复构建**：default 和 classic 两个前端各自独立安装依赖和构建，没有利用缓存
4. **最终镜像使用 `debian:bookworm-slim`**：约 80MB 基础镜像，可以更小
5. **安装了不必要的包**：`libasan8`、`wget` 等可能不需要

## 优化方案

### 1. docker-build-notify.yml：改用原生 arm64 runner（最大收益）

**现状**：`docker-build-notify.yml` 在单个 `ubuntu-latest` 上用 QEMU 同时构建 amd64+arm64，arm64 通过模拟执行，极慢。

**优化**：改为与 `docker-build.yml`/`docker-image-nightly.yml` 相同的分架构原生构建模式——amd64 跑 `ubuntu-latest`，arm64 跑 `ubuntu-24.04-arm`，最后合并 manifest。

**预期收益**：arm64 构建速度提升 5-10 倍，总构建时间从 20-30 分钟降至 5-8 分钟。

### 2. Dockerfile 优化：改善缓存层

**现状**：`COPY . .` 在 Go 构建阶段会复制所有源码（含前端 dist），任何 Go 文件变动都导致 `go build` 缓存完全失效。

**优化**：
- 先 `COPY go.mod go.sum` + `go mod download`（已有）
- 然后只复制 Go 源码目录（排除前端 dist 目录，因为前端 dist 已通过 `--from=builder` 单独复制）
- 将 `COPY . .` 改为只复制必要的 Go 源码目录
- 在 `go build` 之前单独复制前端 dist，确保前端产物变化不使 Go 编译缓存失效

### 3. Dockerfile 优化：最终镜像瘦身

**现状**：`debian:bookworm-slim` + `libasan8` + `wget`

**优化**：
- 换用 `alpine:3.21` 作为运行时镜像（约 3MB vs 80MB）
- 只安装 `ca-certificates` 和 `tzdata`
- 移除 `libasan8`（Address Sanitizer 运行时库，生产环境不需要）和 `wget`（运行时不需要）
- 如果确实需要 wget 健康检查，改用 alpine 自带的 `wget`

### 4. .dockerignore 优化

**现状**：忽略了部分目录但不完整。

**优化**：补充忽略更多不需要的文件（test 文件、IDE 配置、CI 配置等），减小构建上下文。

## 具体改动

### 文件 1：`.github/workflows/docker-build-notify.yml`

改为分架构矩阵构建 + manifest 合并模式：
- 添加 `strategy.matrix`，amd64 跑 `ubuntu-latest`，arm64 跑 `ubuntu-24.04-arm`
- 每个 job 只构建单架构，使用 `--platform ${{ matrix.platform }}`
- 新增 `create_manifests` job 合并多架构 manifest
- 保留邮件通知逻辑

### 文件 2：`Dockerfile`

优化 Go 构建阶段的缓存和最终镜像：
- 用 `alpine:3.21` 替代 `debian:bookworm-slim`
- 移除 `libasan8` 和 `wget`
- 优化 Go 阶段的 COPY 顺序，先复制 go 源码再复制前端 dist
- 添加 `RUN apk add --no-cache ca-certificates tzdata`

### 文件 3：`.dockerignore`

补充忽略项，减小构建上下文体积。

## 预期效果

| 指标 | 优化前 | 优化后 |
|------|--------|--------|
| 构建时间 | 20-30 分钟 | 5-8 分钟 |
| 最终镜像体积 | ~120MB+ | ~30-40MB |
| arm64 构建方式 | QEMU 模拟 | 原生 runner |

## 验证步骤

1. 推送改动到 main 分支，观察 `docker-build-notify` workflow 是否正常触发
2. 检查两个架构的构建是否并行执行
3. 确认 manifest 合并成功
4. 拉取镜像验证功能正常：`docker run --rm -p 3000:3000 <image>` 访问 Web UI
5. 检查镜像体积是否缩小
