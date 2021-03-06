#! /bin/bash

# Look up your GitHub Application Settings to find these
# (default: --none--)
export CLIENT_ID=
export CLIENT_SECRET=
# Random string to used to identify sessions (32 characters long)
# (default: --none--)
export SESSION_AUTH=
# Random string to used to encrypt sessions (32 characters long)
# (default: --none--)
export SESSION_ENC=
# File system path where the platform may store persistent files
# (default: cache)
export CACHE_PATH=cache
# GitHub user name of the person who administrates the platform
# (default: --none--)
export ADMIN_USER=
# Host where the application is reachable
# (default: --none--)
export APP_HOST=
# Port where the application is reachable
# (default: 8080)
export APP_PORT=8080
# Subdirectory where the application is reachable (should start and end with /)
# (default: /)
export APP_SUBDIR=/
# Use https instead of http for the platform?
# (default: false)
export APP_SSL_MODE=false
# Port where the worker interface is exposed
# (default: 4242)
export WORKER_PORT=4242
# Host name where the postgreSQL database is located
# (default: localhost)
export DB_HOST=localhost
# User that is used to access the PostgreSQL database
# (default: analysisbots)
export DB_USER=analysisbots
# Password that is used to access the PostgreSQL database
# (default: --none--)
export DB_PASS=
# Name of the database that is used to store the platforms data
# (default: analysisbots)
export DB_NAME=analysisbots
