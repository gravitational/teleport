CREATE ROLE testuser LOGIN;

CREATE SCHEMA antithesis;

CREATE TABLE antithesis.seed_items (
    id integer PRIMARY KEY,
    name text NOT NULL,
    active boolean NOT NULL DEFAULT true
);

INSERT INTO antithesis.seed_items (id, name, active) VALUES
    (1, 'alpha', true),
    (2, 'bravo', true),
    (3, 'charlie', false);

CREATE TABLE antithesis.workload_markers (
    marker text PRIMARY KEY,
    source text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

GRANT USAGE ON SCHEMA antithesis TO testuser;
GRANT SELECT ON antithesis.seed_items TO testuser;
GRANT SELECT, INSERT, UPDATE, DELETE ON antithesis.workload_markers TO testuser;
