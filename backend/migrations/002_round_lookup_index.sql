-- Speeds up "latest round of a room" lookups used by GET /rooms/:code/rounds/current.
CREATE INDEX IF NOT EXISTS idx_voting_rounds_room_created ON voting_rounds(room_id, created_at DESC);
