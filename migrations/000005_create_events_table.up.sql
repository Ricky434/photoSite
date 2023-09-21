CREATE TABLE IF NOT exists events (
    id serial PRIMARY KEY,
    name text UNIQUE NOT NULL,
    day date,
    version integer NOT NULL DEFAULT 1
);

ALTER TABLE photos ADD COLUMN IF NOT EXISTS event int NULL;
ALTER TABLE photos ADD CONSTRAINT fk_event_id FOREIGN KEY (event) REFERENCES events(id) ON UPDATE CASCADE;
