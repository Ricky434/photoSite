CREATE TABLE IF NOT EXISTS users (
    id bigserial PRIMARY KEY,
    level smallint NOT NULL DEFAULT 0,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    name text UNIQUE NOT NULL,
    password_hash bytea NOT NULL,
    version integer NOT NULL DEFAULT 1
);
