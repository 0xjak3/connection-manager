CREATE TABLE users (
    id            uuid        PRIMARY KEY,
    email         text        NOT NULL,
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_email_not_blank CHECK (email <> ''),
    CONSTRAINT users_password_hash_not_blank CHECK (password_hash <> '')
);
