ALTER TABLE employee ADD COLUMN IF NOT EXISTS face_enrolled BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS employee_face (
    id BIGSERIAL PRIMARY KEY,
    employee_id UUID NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
    face_descriptor TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    CONSTRAINT unique_employee_face UNIQUE (employee_id)
);
