# Build stage
FROM golang:1.26-alpine AS builder

ARG HTTP_PROXY
ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTP_PROXY}
ENV PROXY_URL=${HTTP_PROXY}

ENV GOPROXY=https://proxy.golang.com.cn,direct

WORKDIR /build
COPY go.mod ./
COPY . .
RUN if [ -f go.sum ]; then go mod download; fi
RUN CGO_ENABLED=0 GOOS=linux go build -o /runner .

# Unset http proxy
ENV HTTP_PROXY=
ENV HTTPS_PROXY=
ENV PROXY_URL=

# Runtime stage
FROM alpine:3.19

ARG DOCKER_VERSION=27.1.0

ARG HTTP_PROXY
ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTP_PROXY}
ENV PROXY_URL=${HTTP_PROXY}

RUN apk add \
    ca-certificates \
    curl

# ===== Docker CLI (可选，用于 DinD 场景) =====
RUN curl -fsSL --http1.1 "https://download.docker.com/linux/static/stable/$(uname -m)/docker-${DOCKER_VERSION}.tgz" \
    | tar -xz -C /usr/local/bin --strip-components=1 docker/docker \
    && chmod +x /usr/local/bin/docker

# Unset http proxy
ENV HTTP_PROXY=
ENV HTTPS_PROXY=
ENV PROXY_URL=

COPY --from=builder /runner /usr/local/bin/runner

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/runner"]
