CREATE TABLE coach_students (
  coach_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  student_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (coach_id, student_id)
);
