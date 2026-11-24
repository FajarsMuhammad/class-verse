-- +goose Up
-- Author is optional in "Tambah buku"; empty means unknown. Title stays required.
ALTER TABLE library_books DROP CONSTRAINT library_books_author_check;
ALTER TABLE library_books ADD CONSTRAINT library_books_author_check CHECK (char_length(author) <= 120);

-- +goose Down
UPDATE library_books SET author = '-' WHERE author = '';
ALTER TABLE library_books DROP CONSTRAINT library_books_author_check;
ALTER TABLE library_books ADD CONSTRAINT library_books_author_check CHECK (char_length(author) BETWEEN 1 AND 120);
