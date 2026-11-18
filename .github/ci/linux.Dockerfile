# Linux toolchain for the "Linux in a container" CI job, which runs on the self-hosted Windows runner
# through Docker. It mirrors what the GitHub-hosted Linux job installs: GTK 4, WebKitGTK 6.0, Go, Node and
# the Wails CLI. The source is not copied in; the job mounts it read-only.
FROM ubuntu:24.04

ARG GO_VERSION
ARG NODE_MAJOR=22
ARG WAILS_VERSION

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates curl git file build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev \
 && curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash - \
 && apt-get install -y --no-install-recommends nodejs \
 && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
ENV PATH=/usr/local/go/bin:/root/go/bin:$PATH

RUN go install "github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION}"
