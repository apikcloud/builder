#!/bin/sh

# odoo-builder-engine is started as a background job (not `exec`'d in its
# place) so this script can still chown /workspace afterwards. That means
# it's not PID 1 and does NOT receive signals docker/podman forward to the
# container on its own — sh must explicitly forward them via trap, or
# Ctrl+C on `odoo-builder build` (invoke.go's invokeContainer) kills only
# the launcher's local docker/podman-run client, leaving this script (and
# the buildctl/buildkitd it started) running server-side until the build
# finishes on its own.
#
# POSIX also mandates that an async command's stdin, unless explicitly
# redirected, is reassigned to /dev/null — silently discarding the
# BuildRequest JSON invoke.go's invokeContainer writes to this container's
# stdin. fd 3 preserves the original stdin across the `&` so `<&3` can
# still hand it to odoo-builder-engine explicitly.
exec 3<&0
odoo-builder-engine <&3 &
engine_pid=$!
trap 'kill -TERM "$engine_pid" 2>/dev/null' INT TERM
wait "$engine_pid"
status=$?

# The container always runs as root (buildkitd's OCI worker needs it for
# mount/overlayfs), so anything it wrote under the mounted /workspace would
# otherwise be left root-owned on the host. HOST_UID/HOST_GID are set by
# internal/launcher.invokeContainer's hostIDEnv when both are resolvable on
# the host.
if [ -n "$HOST_UID" ] && [ -n "$HOST_GID" ]; then
    chown -R "$HOST_UID:$HOST_GID" /workspace/.build 2>/dev/null || true
    # /host-cache/odoo-builder is the host's persistent BuildKit local-cache
    # directory (internal/launcher.invokeContainer bind-mounts it, builder.yaml's
    # cache.enabled writes to it) — same root-ownership problem as
    # /workspace/.build above.
    chown -R "$HOST_UID:$HOST_GID" /host-cache/odoo-builder 2>/dev/null || true
fi

exit "$status"
