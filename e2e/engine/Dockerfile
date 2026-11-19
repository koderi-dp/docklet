# The test engine: Docker-in-Docker plus an SSH server, so Docklet can reach the same engine over TCP and SSH.
FROM docker:29-dind

RUN apk add --no-cache openssh-server openssh-keygen \
    # sshd runs remote commands with a minimal PATH; `docker system dial-stdio` must be found there.
    && ln -s /usr/local/bin/docker /usr/bin/docker

COPY --chmod=755 entrypoint.sh /usr/local/bin/e2e-engine

# No TLS: the engine is only reachable on the private Compose network.
ENV DOCKER_TLS_CERTDIR=""

ENTRYPOINT ["e2e-engine"]
