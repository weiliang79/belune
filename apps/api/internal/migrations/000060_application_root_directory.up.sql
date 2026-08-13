-- The build directory an app's builder (Railpack/buildpacks/Dockerfile) runs
-- from within a cloned Git repo. Today it's always the clone root, so a repo
-- laid out as a monorepo (or a single examples repo with one subdirectory
-- per framework) can't be deployed from anything but the top level. NULL/empty
-- keeps today's behavior — build from the repo root.
ALTER TABLE applications ADD COLUMN root_directory TEXT;
