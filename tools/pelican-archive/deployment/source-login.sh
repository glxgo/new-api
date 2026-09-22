#!/bin/sh
# Root-owned forced SSH entrypoint, installed as /usr/local/lib/pelican-archive/source-login.sh.
set -eu
# Export only on a commandless connection. Reject shell commands and subsystems.
if [ "$#" -ne 0 ] || [ -n "${SSH_ORIGINAL_COMMAND:-}" ]; then
    printf '%s\n' 'Only the archive export is available.' >&2
    exit 64
fi
exec /usr/bin/sudo -n /usr/local/sbin/pelican-export
