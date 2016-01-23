FROM postgres:latest

ENV DB_HOST ${DB_HOST:-database}
ENV DB_USER ${DB_USER:-analysisbots}
ENV DB_PASS ${DB_PASS:-YouReallyShouldChangeThis}
ENV DB_NAME ${DB_NAME:-analysisbots}

COPY setup-database.sql /docker-entrypoint-initdb.d/
