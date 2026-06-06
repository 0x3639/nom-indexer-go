-- migrations/016_bridge_time_challenges.up.sql
-- Pending bridge time challenges (the delay window before a security-sensitive
-- bridge method takes effect). Polled from bridge.getTimeChallengesInfo; rows
-- are removed once the challenge is executed or expires.
CREATE TABLE IF NOT EXISTS bridge_time_challenges (
    method_name             TEXT PRIMARY KEY,
    params_hash             TEXT   NOT NULL DEFAULT '',
    challenge_start_height  BIGINT NOT NULL DEFAULT 0,
    delay                   BIGINT NOT NULL DEFAULT 0,
    end_height              BIGINT NOT NULL DEFAULT 0,
    last_updated_timestamp  BIGINT NOT NULL DEFAULT 0
);
