#!/bin/sh
# entrypoint.sh — Fly.io runtime shim for the single-machine demo.
#
# Fly mounts a freshly-created volume at /data at RUNTIME, on top of the
# image's /data directory. A brand-new Fly volume's root is owned by
# root:root, which masks the build-time `chown -R appuser /data` in
# Dockerfile.fly. The backend runs as the unprivileged appuser (uid 1001)
# and, on first boot, creates the embedded-Postgres cluster (/data/pg)
# and the local-storage tree (/data/uploads, /data/tmp, ...). Those
# os.MkdirAll calls fail with EACCES on a root-owned mount.
#
# So we start as root, hand ownership of the mounted volume to appuser,
# then drop privileges with su-exec. The drop is mandatory: embedded
# Postgres' initdb refuses to run as root.
#
# /data/pg-bin is no longer created here: the postgres binary is now
# baked into the image at /usr/lib/postgres (Dockerfile.fly stage 2),
# copied from the tensorchord/vchord-suite image so pgvector, vchord,
# cube, earthdistance, pg_trgm and unaccent are present. Saving the
# download + extraction on every fresh volume avoided a 50 MB write to
# the volume, but more importantly it side-stepped the vanilla binary's
# missing-extension problem that crashed 001_initial_schema.sql.
set -e

mkdir -p /data/pg /data/uploads /data/tmp \
         /data/thumbs /data/profile /data/library /data/video

# Only chown when the mount isn't already owned by appuser, so restarts
# with a large library don't pay a recursive chown every boot.
if [ "$(stat -c %u /data)" != "1001" ]; then
	chown -R appuser:appuser /data
fi

exec gosu appuser /app/immich-go-backend "$@"
