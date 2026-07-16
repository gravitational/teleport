FROM postgres:18.4-trixie

RUN apt-get update \
    && apt-get install -y postgresql-18-wal2json \
    && rm -rf /var/lib/apt/lists/*

# Bake the configs into the image, all SUTs share the same configuration for now. If needed they can
# be overriden with a volume mount instead or custom CMD.
COPY  postgresql.conf /etc/postgresql/postgresql.conf
COPY  pg_hba.conf /etc/postgresql/pg_hba.conf
COPY  init.sql /docker-entrypoint-initdb.d/init.sql

CMD [ "postgres", "-c", "hba_file=/etc/postgresql/pg_hba.conf", "-c", "config_file=/etc/postgresql/postgresql.conf" ]
