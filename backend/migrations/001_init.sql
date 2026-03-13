CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Room status enum
DO $$ BEGIN
    CREATE TYPE room_status AS ENUM ('waiting', 'voting', 'revealed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Round status enum
DO $$ BEGIN
    CREATE TYPE round_status AS ENUM ('voting', 'revealed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Rooms table
CREATE TABLE IF NOT EXISTS rooms (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(8) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    status room_status NOT NULL DEFAULT 'waiting',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Participants table
CREATE TABLE IF NOT EXISTS participants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    display_name VARCHAR(100) NOT NULL,
    session_token UUID UNIQUE NOT NULL DEFAULT uuid_generate_v4(),
    is_host BOOLEAN NOT NULL DEFAULT FALSE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Voting rounds table
CREATE TABLE IF NOT EXISTS voting_rounds (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    story_title VARCHAR(500) NOT NULL,
    status round_status NOT NULL DEFAULT 'voting',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Votes table
CREATE TABLE IF NOT EXISTS votes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    round_id UUID NOT NULL REFERENCES voting_rounds(id) ON DELETE CASCADE,
    participant_id UUID NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    value VARCHAR(10) NOT NULL,
    voted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(round_id, participant_id)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_participants_room_id ON participants(room_id);
CREATE INDEX IF NOT EXISTS idx_participants_session_token ON participants(session_token);
CREATE INDEX IF NOT EXISTS idx_voting_rounds_room_id ON voting_rounds(room_id);
CREATE INDEX IF NOT EXISTS idx_votes_round_id ON votes(round_id);
