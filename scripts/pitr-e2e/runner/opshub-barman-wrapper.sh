#!/usr/bin/env sh
set -eu

if [ "$(id -u)" = "0" ]; then
  restore_dest=""
  after_restore=0
  for arg in "$@"; do
    if [ "$arg" = "restore" ]; then
      after_restore=1
      continue
    fi
    if [ "$after_restore" = "1" ]; then
      after_restore=2
      continue
    fi
    if [ "$after_restore" = "2" ]; then
      after_restore=3
      continue
    fi
    if [ "$after_restore" = "3" ]; then
      restore_dest="$arg"
      break
    fi
  done

  chown -R barman:barman /var/lib/barman /var/log/barman /var/lib/opshub-pitr-e2e 2>/dev/null || true
  if [ -n "$restore_dest" ]; then
    mkdir -p "$restore_dest"
    chown -R barman:barman "$restore_dest" "$(dirname "$restore_dest")" 2>/dev/null || true
  fi

  exec runuser -u barman -- /usr/bin/barman "$@"
fi

exec /usr/bin/barman "$@"
