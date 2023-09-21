CREATE TABLE IF NOT EXISTS photos (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    taken_at timestamp(0) with time zone,
    latitude float CHECK (latitude between -90 and 90),
    longitude float CHECK (longitude between -90 and 90),
    CONSTRAINT valid_coords CHECK ((latitude is not null and longitude is not null) or (latitude is null and longitude is null))
);

CREATE INDEX IF NOT EXISTS index_date ON photos (taken_at NULLS LAST);
CREATE INDEX IF NOT EXISTS index_lat ON photos (latitude NULLS LAST);
CREATE INDEX IF NOT EXISTS index_long ON photos (longitude NULLS LAST);
