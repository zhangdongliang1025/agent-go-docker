ARG RUST_VERSION=1.85.0

# ===== Stage 1: Build shpool =====
FROM rust:${RUST_VERSION}-slim AS shpool-builder

ARG HTTP_PROXY
ARG CARGO_REGISTRIES_CRATES_IO_INDEX=https://github.com/rust-lang/crates.io-index
ARG CARGO_REGISTRIES_CRATES_IO_PROTOCOL=sparse

ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTP_PROXY}
ENV PROXY_URL=${HTTP_PROXY}
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    ca-certificates \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

RUN cargo install --locked shpool --root /usr/local \
    && chmod +x /usr/local/bin/shpool \
    && rm -rf /usr/local/cargo/registry /usr/local/cargo/git

# ===== Stage 2: Base image with all dev environments =====
FROM node:24-slim

ARG USER_UID=1000
ARG USER_GID=1000
ARG TARGETARCH
ARG DOCKER_VERSION=27.1.0
ARG TTYD_VERSION="1.7.7"
ARG GIT_VERSION=2.49.1

ARG HTTP_PROXY
ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTP_PROXY}
ENV PROXY_URL=${HTTP_PROXY}

ENV DEBIAN_FRONTEND=noninteractive
ENV SHELL=/bin/bash

# ===== Install base tools =====
RUN  apt-get update && apt-get install -y \
    curl \
    wget \
    vim \
    neovim \
    ripgrep \
    fd-find \
    jq \
    tree \
    htop \
    build-essential \
    openssh-client \
    ca-certificates \
    sudo \
    tzdata \
    locales \
    zlib1g-dev \
    libssl-dev \
    libcurl4-openssl-dev \
    libexpat1-dev \
    gettext \
    tcl \
    && rm -rf /var/lib/apt/lists/*

# ===== Install Git =====
RUN curl -fsSL --http1.1 "https://www.kernel.org/pub/software/scm/git/git-${GIT_VERSION}.tar.gz" \
    -o /tmp/git.tar.gz \
    && tar -xzf /tmp/git.tar.gz -C /tmp \
    && make -C "/tmp/git-${GIT_VERSION}" prefix=/usr/local all \
    && make -C "/tmp/git-${GIT_VERSION}" prefix=/usr/local install \
    && rm -rf "/tmp/git-${GIT_VERSION}" /tmp/git.tar.gz

# ===== Install ttyd =====
RUN ARCH=$(uname -m) && \
    case ${ARCH} in \
        x86_64)  TTYD_ARCH="x86_64" ;; \
        aarch64) TTYD_ARCH="aarch64"  ;; \
        *)       echo "Unsupported architecture: ${ARCH}"; exit 1 ;; \
    esac && \
    TTYD_URL="https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.${TTYD_ARCH}" && \
    wget -O /usr/local/bin/ttyd ${TTYD_URL} && \
    chmod +x /usr/local/bin/ttyd

ENV PATH="/usr/local/bin:${PATH}"

# ===== Set locale =====
RUN sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen && locale-gen
ENV LANG=en_US.UTF-8
ENV LANGUAGE=en_US:en
ENV LC_ALL=en_US.UTF-8

# ===== Install Python3 =====
RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*

# ===== Install Docker CLI =====
RUN curl -fsSL --http1.1 "https://download.docker.com/linux/static/stable/$(uname -m)/docker-${DOCKER_VERSION}.tgz" \
    | tar -xz -C /usr/local/bin --strip-components=1 docker/docker \
    && chmod +x /usr/local/bin/docker

RUN groupadd --gid 999 docker && usermod -aG docker node

# ===== Copy shpool from builder =====
COPY --from=shpool-builder /usr/local/bin/shpool /usr/local/bin/shpool

# ===== Configure shpool =====
RUN mkdir -p /etc/shpool && \
    printf 'output_spool_lines = 20000\nsession_restore_mode = { lines = 20000 }\n' > /etc/shpool/config.toml && \
    chmod 0644 /etc/shpool/config.toml

# ===== Install Claude Code =====
RUN npm install -g @anthropic-ai/claude-code

# ===== Install Codex =====
RUN npm i -g @openai/codex

# ===== Install gosu for UID mapping =====
RUN ARCH=${TARGETARCH:-$(dpkg --print-architecture)} && \
    curl -fsSL --http1.1 "https://github.com/tianon/gosu/releases/download/1.17/gosu-${ARCH}" -o /usr/local/bin/gosu && \
    chmod +x /usr/local/bin/gosu

# ===== Install uv (Python package manager) =====
RUN curl -LsSf https://astral.sh/uv/install.sh | env UV_INSTALL_DIR=/usr/local/bin sh

# ===== Install multiple JDK versions via SDKMAN! =====
RUN apt-get update && apt-get install -y \
    zip \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# Install SDKMAN!
RUN curl -s "https://get.sdkman.io" | bash

# Install JDKs via SDKMAN! (8, 11, 17, 21, 25)
ENV SDKMAN_DIR="/usr/local/sdkman"
RUN bash -c "source /usr/local/sdkman/bin/sdkman-init.sh && \
    sdk install java 8.0.442-tem && \
    sdk install java 11.0.26-tem && \
    sdk install java 17.0.14-tem && \
    sdk install java 21.0.6-tem && \
    sdk install java 25.0.0-tem"

# Set default JDK to 17
ENV JAVA_HOME="/usr/local/sdkman/candidates/java/current"
ENV PATH="${JAVA_HOME}/bin:${PATH}"
RUN bash -c "source /usr/local/sdkman/bin/sdkman-init.sh && sdk default java 17.0.14-tem"

# ===== Install Maven =====
RUN bash -c "source /usr/local/sdkman/bin/sdkman-init.sh && sdk install maven"
ENV MAVEN_HOME="/usr/local/sdkman/candidates/maven/current"
ENV PATH="${MAVEN_HOME}/bin:${PATH}"

# ===== Install Go =====
RUN GO_VERSION=$(curl -s https://go.dev/VERSION?m=text | head -n 1) && \
    GOARCH=$(dpkg --print-architecture) && \
    curl -fsSL "https://go.dev/dl/${GO_VERSION}.linux-${GOARCH}.tar.gz" | tar -C /usr/local -xzf - && \
    ln -sf /usr/local/go/bin/go /usr/local/bin/go && \
    ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
ENV GOPATH="/home/node/go"
ENV PATH="/usr/local/go/bin:${GOPATH}/bin:${PATH}"

# ===== Install Rust =====
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | \
    RUSTUP_HOME=/usr/local/rustup CARGO_HOME=/usr/local/cargo sh -s -- -y && \
    chmod -R a+w /usr/local/cargo /usr/local/rustup
ENV RUSTUP_HOME="/usr/local/rustup"
ENV CARGO_HOME="/usr/local/cargo"
ENV PATH="/usr/local/cargo/bin:${PATH}"

# ===== Unset http proxy =====
ENV HTTP_PROXY=
ENV HTTPS_PROXY=
ENV PROXY_URL=

# ===== Copy entrypoint =====
COPY entrypoint.sh /entrypoint.sh

# ===== Setup node user =====
WORKDIR /home/node
RUN git config --global init.defaultBranch main \
    && chown -R node:node /home/node

EXPOSE 7681

ENTRYPOINT ["/entrypoint.sh"]
CMD ["claude"]
