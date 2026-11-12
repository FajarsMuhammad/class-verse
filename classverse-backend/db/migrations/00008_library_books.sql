-- +goose Up
-- Draft: published_at NULL. Archived rows stay for history and keep their file reference.
CREATE TABLE library_books (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES organizations(id),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    author text NOT NULL CHECK (char_length(author) BETWEEN 1 AND 120),
    -- Same keys as the library shelves in the frontend's rooms.json.
    shelf text NOT NULL CHECK (shelf IN ('dongeng', 'novel', 'komik', 'sains', 'matematika', 'sejarah', 'inggris', 'referensi')),
    level text NOT NULL CHECK (level IN ('SD', 'SMP', 'SMA', 'Umum')),
    format text NOT NULL CHECK (format IN ('buku', 'ebook', 'audio')),
    copies integer NOT NULL DEFAULT 0 CHECK (copies BETWEEN 0 AND 999),
    file_id uuid,
    cover text NOT NULL CHECK (cover IN ('ipa', 'matematika', 'sejarah', 'bahasa', 'kreatif', 'diskusi', 'kelas', 'arena')),
    sample boolean NOT NULL DEFAULT false,
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    published_at timestamptz,
    archived_at timestamptz,
    FOREIGN KEY (org_id, file_id) REFERENCES files(org_id, id),
    CHECK ((format = 'buku') = (file_id IS NULL)),
    CHECK (format = 'buku' OR copies = 0)
);
CREATE INDEX library_books_catalog_idx ON library_books (org_id, title, id) WHERE archived_at IS NULL;
CREATE INDEX library_books_file_idx ON library_books (org_id, file_id) WHERE archived_at IS NULL;

-- +goose Down
DROP TABLE library_books;
