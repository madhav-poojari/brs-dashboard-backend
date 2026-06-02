-- Refactor relations table: composite PK (coach_id, user_id) -> single PK (user_id)
-- Now: student rows have user_id=student, coach_id=coach, mentor_id=mentor
--       coach rows have user_id=coach, coach_id='', mentor_id=mentor

BEGIN;

CREATE TABLE relations_new (
    user_id  TEXT PRIMARY KEY,
    coach_id TEXT DEFAULT '',
    mentor_id TEXT DEFAULT ''
);

-- Copy student rows (skip T- tracking rows)
INSERT INTO relations_new (user_id, coach_id, mentor_id)
SELECT DISTINCT ON (user_id) user_id, coach_id, COALESCE(mentor_id, '')
FROM relations
WHERE user_id NOT LIKE 'T-%'
ORDER BY user_id;

-- Create coach-mentor rows from tracking data
INSERT INTO relations_new (user_id, coach_id, mentor_id)
SELECT DISTINCT ON (coach_id) coach_id, '', COALESCE(mentor_id, '')
FROM relations
WHERE mentor_id IS NOT NULL AND mentor_id != ''
  AND coach_id NOT IN (SELECT user_id FROM relations_new)
ORDER BY coach_id
ON CONFLICT (user_id) DO NOTHING;

DROP TABLE relations;
ALTER TABLE relations_new RENAME TO relations;

CREATE INDEX idx_relations_coach_id ON relations (coach_id);
CREATE INDEX idx_relations_mentor_id ON relations (mentor_id);

COMMIT;
