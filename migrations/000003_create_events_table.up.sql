CREATE TABLE IF NOT exists events (
    id serial PRIMARY KEY,
    name text UNIQUE NOT NULL,
    day date,
    version integer NOT NULL DEFAULT 1
);
