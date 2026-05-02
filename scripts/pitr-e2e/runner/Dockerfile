ARG POSTGRES_IMAGE=postgres:16
FROM ${POSTGRES_IMAGE}

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    barman \
    ca-certificates \
    jq \
    openssh-server \
    procps \
    rsync \
    sudo \
  && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /run/sshd /var/lib/barman /var/log/barman /etc/barman.d /var/lib/opshub-pitr-e2e/runner \
  && chown -R barman:barman /var/lib/barman /var/log/barman /var/lib/opshub-pitr-e2e \
  && sed -i 's/^#\?PermitRootLogin .*/PermitRootLogin yes/' /etc/ssh/sshd_config \
  && sed -i 's/^#\?PasswordAuthentication .*/PasswordAuthentication yes/' /etc/ssh/sshd_config \
  && sed -i 's/^#\?UsePAM .*/UsePAM no/' /etc/ssh/sshd_config

COPY barman.conf /etc/barman.conf
COPY entrypoint.sh /entrypoint.sh
COPY opshub-barman-wrapper.sh /usr/local/bin/barman

RUN chmod 0644 /etc/barman.conf \
  && chmod 0755 /entrypoint.sh /usr/local/bin/barman

CMD ["/entrypoint.sh"]
