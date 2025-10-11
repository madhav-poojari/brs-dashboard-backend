CREATE TABLE user_details (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  city TEXT,
  state TEXT,
  country TEXT,
  zipcode TEXT,
  phone TEXT,
  dob DATE,
  lichess_username TEXT,
  uscf_id TEXT,
  chesscom_username TEXT,
  fide_id TEXT,
  bio TEXT,
  profile_picture_url TEXT,
  additional_info JSONB DEFAULT '{}'::jsonb,
  updated_at TIMESTAMPTZ DEFAULT now()
);
