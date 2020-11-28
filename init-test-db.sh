#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE EXTENSION pgcrypto;
    CREATE ROLE test;
    CREATE TABLE users (username text, password text);
    INSERT INTO users VALUES ('test', crypt('test', gen_salt('bf', 8)));
    INSERT INTO users VALUES ('postgres', crypt('test', gen_salt('bf', 8)));
EOSQL
