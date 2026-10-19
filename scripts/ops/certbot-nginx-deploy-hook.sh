#!/bin/sh
# Install as a root-owned executable in Certbot's renewal-hooks/deploy directory.
# Never reload Nginx after a failed configuration or certificate-pair check.
set -eu

/usr/sbin/nginx -t
/usr/bin/systemctl reload nginx
