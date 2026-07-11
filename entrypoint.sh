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

# Demo-instance fresh start: when IMMICH_DEMO_FRESH_ON_DEPLOY=true, wipe
# all persistent state exactly once per new image release. Fly sets
# FLY_IMAGE_REF to the deployed image reference; we stamp it into
# /data/.deploy-stamp after a wipe. A machine *restart* (OOM, host
# migration) keeps the same image ref and therefore keeps its data —
# only a `fly deploy` with a new image triggers the reset.
if [ "$IMMICH_DEMO_FRESH_ON_DEPLOY" = "true" ] && [ -n "$FLY_IMAGE_REF" ]; then
	stamp_file=/data/.deploy-stamp
	current_stamp="$(cat "$stamp_file" 2>/dev/null || true)"
	if [ "$current_stamp" != "$FLY_IMAGE_REF" ]; then
		echo "entrypoint: new release ($FLY_IMAGE_REF), wiping demo state" >&2
		rm -rf /data/pg /data/uploads /data/tmp /data/thumbs \
		       /data/profile /data/library /data/video
		printf '%s' "$FLY_IMAGE_REF" > "$stamp_file"
	fi
fi

mkdir -p /data/pg /data/uploads /data/tmp \
         /data/thumbs /data/profile /data/library /data/video

# Only chown when the mount isn't already owned by appuser, so restarts
# with a large library don't pay a recursive chown every boot.
if [ "$(stat -c %u /data)" != "1001" ]; then
	chown -R appuser:appuser /data
fi

# The recursive chown above is skipped when /data itself is already
# appuser-owned, but a demo wipe recreates the subdirectories as root.
# Non-recursive chown of the top-level dirs is cheap and always safe.
chown appuser:appuser /data/pg /data/uploads /data/tmp \
	/data/thumbs /data/profile /data/library /data/video

exec gosu appuser /app/immich-go-backend "$@"
