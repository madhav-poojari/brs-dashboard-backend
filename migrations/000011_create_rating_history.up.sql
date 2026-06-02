CREATE TABLE rating_history (
  id          SERIAL PRIMARY KEY,
  user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  platform    TEXT NOT NULL,
  rating_type TEXT NOT NULL,
  rating      INTEGER NOT NULL,
  recorded_at TIMESTAMPTZ NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rating_history_user_platform ON rating_history (user_id, platform);
CREATE INDEX idx_rating_history_recorded_at ON rating_history (recorded_at);
