CREATE ROLE opshub WITH LOGIN SUPERUSER PASSWORD 'opshub';
CREATE ROLE barman WITH LOGIN SUPERUSER REPLICATION PASSWORD 'barman';

CREATE DATABASE opshub_pitr OWNER opshub;

\connect opshub_pitr

CREATE TABLE IF NOT EXISTS pitr_marker (
  id BIGSERIAL PRIMARY KEY,
  label TEXT NOT NULL UNIQUE,
  payload TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

ALTER TABLE pitr_marker OWNER TO opshub;
