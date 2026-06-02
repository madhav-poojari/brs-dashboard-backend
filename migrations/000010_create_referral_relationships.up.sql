CREATE TYPE relationship_type AS ENUM (
  'vendor',
  'classmate_college',
  'classmate_school',
  'coworker',
  'family',
  'friend',
  'coach',
  'student',
  'other'
);

CREATE TABLE referral_relationships (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  referrer_id VARCHAR(10) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  referee_id VARCHAR(10) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  relationship_type relationship_type NOT NULL,
  relationship_description TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  created_by VARCHAR(10) REFERENCES users(id) ON DELETE SET NULL,
  
  CONSTRAINT no_self_referral CHECK (referrer_id != referee_id),
  CONSTRAINT unique_relationship UNIQUE (referrer_id, referee_id)
);

CREATE INDEX idx_referral_referrer ON referral_relationships(referrer_id);
CREATE INDEX idx_referral_referee ON referral_relationships(referee_id);
CREATE INDEX idx_referral_type ON referral_relationships(relationship_type);
CREATE INDEX idx_referral_created_by ON referral_relationships(created_by);
CREATE INDEX idx_referral_created_at ON referral_relationships(created_at);