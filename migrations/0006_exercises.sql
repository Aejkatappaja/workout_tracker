-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pg_trgm;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE exercises (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    muscle_group TEXT NOT NULL DEFAULT 'other'
        CHECK (muscle_group IN ('chest','back','legs','shoulders','arms','core','cardio','other')),
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_exercises_name_trgm ON exercises USING gin (name gin_trgm_ops);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO exercises (name, muscle_group) VALUES
    ('bench press','chest'),
    ('incline bench press','chest'),
    ('push up','chest'),
    ('dumbbell fly','chest'),
    ('back squat','legs'),
    ('front squat','legs'),
    ('leg press','legs'),
    ('lunge','legs'),
    ('romanian deadlift','legs'),
    ('deadlift','back'),
    ('barbell row','back'),
    ('pull up','back'),
    ('lat pulldown','back'),
    ('overhead press','shoulders'),
    ('lateral raise','shoulders'),
    ('face pull','shoulders'),
    ('bicep curl','arms'),
    ('tricep pushdown','arms'),
    ('hammer curl','arms'),
    ('plank','core'),
    ('hanging leg raise','core'),
    ('running','cardio'),
    ('rowing','cardio'),
    ('cycling','cardio'),
    ('jump rope','cardio')
ON CONFLICT (name) DO NOTHING;
-- +goose StatementEnd

-- normalize workout_entries: free-text exercise_name -> exercises FK
-- +goose StatementBegin
ALTER TABLE workout_entries ADD COLUMN exercise_id INTEGER REFERENCES exercises(id);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO exercises (name)
SELECT DISTINCT lower(trim(exercise_name))
FROM workout_entries
WHERE exercise_name <> ''
ON CONFLICT (name) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE workout_entries e
SET exercise_id = x.id
FROM exercises x
WHERE lower(trim(e.exercise_name)) = x.name;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE workout_entries ALTER COLUMN exercise_id SET NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE workout_entries DROP COLUMN exercise_name;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workout_entries ADD COLUMN exercise_name TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE workout_entries e
SET exercise_name = x.name
FROM exercises x
WHERE e.exercise_id = x.id;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE workout_entries DROP COLUMN exercise_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE exercises;
-- +goose StatementEnd
