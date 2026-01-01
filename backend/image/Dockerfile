FROM ubuntu:22.04

# install basic tools
RUN apt-get update && apt-get install -y \
    ca-certificates \
    git \
    curl \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# install pack CLI
RUN curl -sSL https://github.com/buildpacks/pack/releases/download/v0.34.4/pack-v0.34.4-linux.tgz \
    | tar -xz -C /usr/local/bin pack

# install CNB lifecycle
ENV CNB_LIFECYCLE_VERSION=0.18.3
RUN curl -sSL https://github.com/buildpacks/lifecycle/releases/download/v${CNB_LIFECYCLE_VERSION}/lifecycle-v${CNB_LIFECYCLE_VERSION}+linux.x86_64.tgz \
    | tar -xz -C /cnb

ENV PATH="/cnb/lifecycle:${PATH}"

# non-root user (important)
RUN useradd -m builder
USER builder
WORKDIR /workspace
