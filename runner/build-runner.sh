REGISTRY="registry.server.gingkoo/ai/agent-run"
PROXY="${HTTP_PROXY:-http://10.1.2.12:8118}"
PLATFORM="linux/amd64,linux/arm64"

docker buildx build --platform "${PLATFORM}"  --build-arg HTTP_PROXY="${PROXY}"  \
 -t "registry.server.gingkoo/ai/agent-run:latest" --push .
