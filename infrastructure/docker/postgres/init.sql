-- TenzoShare PostgreSQL Initialization
-- Runs once when the container is first created.
-- Service-specific schemas are applied via individual service migrations.

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";   -- Full-text trigram search

-- Create service-specific databases (each service owns its own schema)
-- All services share the same PostgreSQL instance but use separate schemas
-- to enforce data ownership boundaries.

CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS transfer;
CREATE SCHEMA IF NOT EXISTS storage;
CREATE SCHEMA IF NOT EXISTS audit;
CREATE SCHEMA IF NOT EXISTS admin_svc;

-- Grant the application user permissions on all schemas
DO $$
DECLARE
  app_user TEXT := current_user;
BEGIN
  EXECUTE format('GRANT ALL ON SCHEMA auth TO %I', app_user);
  EXECUTE format('GRANT ALL ON SCHEMA transfer TO %I', app_user);
  EXECUTE format('GRANT ALL ON SCHEMA storage TO %I', app_user);
  EXECUTE format('GRANT ALL ON SCHEMA audit TO %I', app_user);
  EXECUTE format('GRANT ALL ON SCHEMA admin_svc TO %I', app_user);
END
$$;
