CREATE TABLE IF NOT EXISTS attendances (
  id BIGSERIAL PRIMARY KEY,
  student_id TEXT NOT NULL,
  coach_id TEXT NOT NULL,
  class_type TEXT NOT NULL,
  date DATE NOT NULL,
  session_id TEXT,
  is_verified BOOLEAN NOT NULL DEFAULT FALSE,
  class_highlights TEXT,
  homework TEXT,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_attendances_student_id ON attendances(student_id);
CREATE INDEX IF NOT EXISTS idx_attendances_coach_id ON attendances(coach_id);
CREATE INDEX IF NOT EXISTS idx_attendances_date ON attendances(date);
CREATE INDEX IF NOT EXISTS idx_attendances_session_id ON attendances(session_id);
CREATE INDEX IF NOT EXISTS idx_attendances_is_verified ON attendances(is_verified);
