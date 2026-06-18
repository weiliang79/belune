-- Phase 6: managed-database resource limits + external-access host port.
--   cpu_limit / memory_limit mirror the applications table (0 = unlimited) and
--     are applied live via `docker update`.
--   host_port is the loopback (127.0.0.1) port bound at provision for SSH-tunnel
--     access; NULL means the database is not bound to the host.
ALTER TABLE databases
    ADD COLUMN cpu_limit    DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (cpu_limit >= 0),
    ADD COLUMN memory_limit BIGINT           NOT NULL DEFAULT 0 CHECK (memory_limit >= 0),
    ADD COLUMN host_port    INTEGER;
