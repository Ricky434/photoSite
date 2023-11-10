ALTER table  photos 
    DROP CONSTRAINT IF EXISTS fk_event_id,
    ADD CONSTRAINT fk_event_id FOREIGN KEY(event) REFERENCES events(id) ON DELETE CASCADE;
