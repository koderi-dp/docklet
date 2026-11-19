# Docklet in Wails server mode. The binary is cross-compiled on the host first (see `task e2e:build`), so this
# image only adds what Docklet runs as child processes: the Docker CLI with Compose, and the ssh client.
FROM docker:29-cli

# dbus: Docklet connects to a session bus at startup (desktop notifications); a private one is enough.
RUN apk add --no-cache openssh-client dbus

COPY --chmod=755 .build/docklet-server /usr/local/bin/docklet-server
COPY --chmod=755 docklet/entrypoint.sh /usr/local/bin/e2e-docklet

# Listen on all interfaces inside the container; Compose publishes the port to localhost only.
ENV WAILS_SERVER_HOST=0.0.0.0 \
    WAILS_SERVER_PORT=8080

EXPOSE 8080
ENTRYPOINT ["e2e-docklet"]
