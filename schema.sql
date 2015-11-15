CREATE TABLE moods (
       id SERIAL,
       created_at TIMESTAMP NOT NULL DEFAULT NOW(),

       user_id TEXT NOT NULL,
       name    TEXT NOT NULL,
       eyes    CHAR(2) NOT NULL,
       tongue  CHAR(2) NOT NULL,

       PRIMARY KEY (id)
);

CREATE UNIQUE INDEX unique_user_moods ON moods (user_id, lower(name));

CREATE TABLE conversations (
       id SERIAL,
       public_id TEXT NOT NULL,
       created_at TIMESTAMP NOT NULL DEFAULT NOW(),

       heading TEXT NOT NULL,
       user_id TEXT NOT NULL,

       UNIQUE(public_id),
       PRIMARY KEY (id)
);

CREATE TABLE lines (
       id SERIAL,
       public_id TEXT NOT NULL,
       created_at TIMESTAMP NOT NULL DEFAULT NOW(),

       animal TEXT NOT NULL,
       text TEXT NOT NULL,
       think BOOLEAN NOT NULL,
       mood_id INTEGER NOT NULL,
       conversation_id INTEGER NOT NULL,

       UNIQUE(public_id),
       PRIMARY KEY (id)
);

ALTER TABLE lines ADD CONSTRAINT fk_lines_conversation
  FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE;

ALTER TABLE lines ADD CONSTRAINT fk_lines_mood
  FOREIGN KEY (mood_id) REFERENCES moods(id);
