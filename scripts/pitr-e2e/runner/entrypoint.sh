#!/usr/bin/env bash
set -Eeuo pipefail

ssh_port="${PITR_SSH_PORT:-2222}"
ssh_password="${PITR_SSH_PASSWORD:-opshub}"

echo "root:${ssh_password}" | chpasswd
mkdir -p /run/sshd /var/lib/barman /var/log/barman /var/lib/opshub-pitr-e2e/runner
chown -R barman:barman /var/lib/barman /var/log/barman /var/lib/opshub-pitr-e2e

exec /usr/sbin/sshd -D -e -p "${ssh_port}"
