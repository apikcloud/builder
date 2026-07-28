#!/bin/sh

odoo-builder-engine
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
