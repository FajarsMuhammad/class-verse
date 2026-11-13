-- +goose Up
-- L2: physical-book loans. Stock = library_books.copies minus active loans, checked in a transaction that
-- locks the borrower's membership row, then the book row (fixed order, no deadlock).
ALTER TABLE library_books ADD CONSTRAINT library_books_org_id_id_key UNIQUE (org_id, id);

CREATE TABLE library_loans (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    book_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id),
    borrowed_at timestamptz NOT NULL,
    due_at timestamptz NOT NULL,
    returned_at timestamptz,
    FOREIGN KEY (org_id, book_id) REFERENCES library_books(org_id, id),
    CHECK (due_at > borrowed_at),
    CHECK (returned_at IS NULL OR returned_at >= borrowed_at)
);
-- One active loan per member and book: a retried borrow returns the existing loan.
CREATE UNIQUE INDEX library_loans_active_idx ON library_loans (book_id, user_id) WHERE returned_at IS NULL;
CREATE INDEX library_loans_user_idx ON library_loans (org_id, user_id) WHERE returned_at IS NULL;

-- +goose Down
DROP TABLE library_loans;
ALTER TABLE library_books DROP CONSTRAINT library_books_org_id_id_key;
