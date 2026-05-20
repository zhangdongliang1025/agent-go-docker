#!/bin/bash
set -e

REGISTRY="registry.server.gingkoo/ai/agent-go-docker"
PROXY="${HTTP_PROXY:-http://10.1.2.12:8118}"
PLATFORM="linux/amd64,linux/arm64"

# ===== 构建并推送多架构镜像 =====
build_and_push() {
    local dockerfile=$1
    local tag=$2
    echo "----------build ${REGISTRY}:${tag} by ${dockerfile}----------"
    http_proxy="${PROXY}" https_proxy="${PROXY}" docker buildx build --platform "${PLATFORM}" \
         -f "${dockerfile}" \
         --build-arg CARGO_REGISTRIES_CRATES_IO_INDEX=https://mirrors.ustc.edu.cn/crates.io-index \
         --build-arg CARGO_REGISTRIES_CRATES_IO_PROTOCOL=sparse \
         --build-arg BASE_IMAGE_REGISTRY="${REGISTRY}" \
         --build-arg HTTP_PROXY="${PROXY}" \
         -t "${REGISTRY}:${tag}" --push .
}

# ===== 主流程 =====
# 1. 先构建基础镜像 latest
build_and_push Dockerfile    "latest"

# 2. 基于 latest 构建变体镜像
build_and_push Dockerfile.java8   "java8"
build_and_push Dockerfile.java17  "java17"
build_and_push Dockerfile.java21  "java21"
build_and_push Dockerfile.java25  "java25"
build_and_push Dockerfile.go      "go"
build_and_push Dockerfile.rust    "rust"

cd runner && http_proxy="${PROXY}" https_proxy="${PROXY}" docker buildx build --platform "${PLATFORM}" \
         --build-arg HTTP_PROXY="${PROXY}" \
         -t "registry.server.gingkoo/ai/agent-run:latest" --push .

echo "=========================================="
echo "==> 全部构建完成!"
echo "=========================================="
