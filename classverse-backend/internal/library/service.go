// Package library manages a school's book catalog (L1): drafts, publishing, archive.
package library

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Allowed values mirror the CHECK constraints in 00008_library_books.sql.
var (
	Shelves = []string{"dongeng", "novel", "komik", "sains", "matematika", "sejarah", "inggris", "referensi"}
	Levels  = []string{"SD", "SMP", "SMA", "Umum"}
	Formats = []string{"buku", "ebook", "audio"}
	Covers  = []string{"ipa", "matematika", "sejarah", "bahasa", "kreatif", "diskusi", "kelas", "arena"}
)

const (
	defaultLimit = 20
	maxLimit     = 50
	maxCopies    = 999
)

// Domain errors.
var (
	ErrNotFound      = errors.New("book not found")
	ErrInvalidCursor = errors.New("invalid cursor")
)

// ValidationError lists invalid input fields.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "invalid book fields" }

func (e *ValidationError) add(field, msg string) {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	e.Fields[field] = msg
}

// Authorizer checks a user's role in an organization (implemented by org.Service).
type Authorizer interface {
	Authorize(ctx context.Context, userID, orgID uuid.UUID, minRole org.Role) (org.Role, error)
}

// Input is the editable part of a book.
type Input struct {
	Title     string
	Author    string
	Shelf     string
	Level     string
	Format    string
	Copies    int
	FileID    *uuid.UUID
	Cover     string
	Published bool
}

// Book is a catalog entry plus the caller's reader position and loan state.
type Book struct {
	query.LibraryBook
	Position int32
	OnLoan   int32
	MyLoan   *query.LibraryLoan
}

// Page is one catalog page.
type Page struct {
	Items      []Book
	NextCursor string
}

// Filter narrows a catalog listing.
type Filter struct {
	Shelf  string
	Query  string
	Limit  int
	Cursor string
	Drafts bool
	Format string // "", buku, ebook, audio
	Level  string // "", SD, SMP, SMA, Umum
	Status string // "", published, draft (draft only for staff with Drafts)
}

// Counts are the staff catalog tab counts.
type Counts = query.CountBooksRow

// validate checks the shared catalog filters.
func (f Filter) validate() error {
	fields := map[string]string{}
	if f.Shelf != "" && !slices.Contains(Shelves, f.Shelf) {
		fields["shelf"] = "Rak tidak dikenal"
	}
	if f.Format != "" && !slices.Contains(Formats, f.Format) {
		fields["format"] = "Jenis tidak dikenal"
	}
	if f.Level != "" && !slices.Contains(Levels, f.Level) {
		fields["level"] = "Jenjang tidak dikenal"
	}
	if f.Status != "" && f.Status != "published" && f.Status != "draft" {
		fields["status"] = "Status tidak valid"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func optText(v string) pgtype.Text { return pgtype.Text{String: v, Valid: v != ""} }

func (f Filter) pattern() pgtype.Text {
	if q, ok := text(f.Query, 1, 100); ok {
		return pgtype.Text{String: "%" + likeEscape(q) + "%", Valid: true}
	}
	return pgtype.Text{}
}

// Counts returns catalog tab counts for the list's non-status filters (teacher/admin).
func (s *Service) Counts(ctx context.Context, userID, orgID uuid.UUID, f Filter) (Counts, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return Counts{}, err
	}
	if err := f.validate(); err != nil {
		return Counts{}, err
	}
	c, err := query.New(s.pool).CountBooks(ctx, query.CountBooksParams{
		OrgID: orgID, Shelf: optText(f.Shelf), Format: optText(f.Format), Level: optText(f.Level), Pattern: f.pattern(),
	})
	if err != nil {
		return Counts{}, fmt.Errorf("count books: %w", err)
	}
	return c, nil
}

// Service implements catalog rules.
type Service struct {
	pool *pgxpool.Pool
	auth Authorizer
	now  func() time.Time
}

// NewService wires the catalog.
func NewService(pool *pgxpool.Pool, auth Authorizer, now func() time.Time) *Service {
	return &Service{pool: pool, auth: auth, now: now}
}

func text(s string, lo, hi int) (string, bool) {
	s = strings.TrimSpace(strings.ToValidUTF8(s, ""))
	n := utf8.RuneCountInString(s)
	return s, n >= lo && n <= hi && !strings.ContainsFunc(s, unicode.IsControl)
}

// validate normalizes input; file checks need the database and happen in checkFile.
func validate(in Input) (Input, error) {
	var v ValidationError
	var ok bool
	if in.Title, ok = text(in.Title, 1, 200); !ok {
		v.add("title", "Judul 1–200 karakter")
	}
	if in.Author, ok = text(in.Author, 1, 120); !ok {
		v.add("author", "Penulis 1–120 karakter")
	}
	if !slices.Contains(Shelves, in.Shelf) {
		v.add("shelf", "Pilih rak")
	}
	if !slices.Contains(Levels, in.Level) {
		v.add("level", "Pilih jenjang")
	}
	if !slices.Contains(Covers, in.Cover) {
		v.add("cover", "Pilih warna sampul")
	}
	switch in.Format {
	case "buku":
		if in.FileID != nil {
			v.add("file_id", "Buku fisik tidak memakai file")
		}
		if in.Copies < 0 || in.Copies > maxCopies {
			v.add("copies", "Jumlah eksemplar 0–999")
		}
	case "ebook", "audio":
		if in.FileID == nil {
			v.add("file_id", "Pilih file")
		}
		// Digital copies are unlimited; L2 loans apply only to physical books.
		in.Copies = 0
	default:
		v.add("format", "Pilih format")
	}
	if v.Fields != nil {
		return in, &v
	}
	return in, nil
}

// checkFile requires a ready file of this school whose kind matches the format.
func checkFile(ctx context.Context, q *query.Queries, orgID uuid.UUID, in Input) error {
	if in.FileID == nil {
		return nil
	}
	f, err := q.GetReadyFile(ctx, query.GetReadyFileParams{OrgID: orgID, ID: *in.FileID})
	if errors.Is(err, pgx.ErrNoRows) {
		return &ValidationError{Fields: map[string]string{"file_id": "File tidak ditemukan atau belum siap"}}
	}
	if err != nil {
		return fmt.Errorf("get book file: %w", err)
	}
	if want := map[string]string{"ebook": "pdf", "audio": "audio"}[in.Format]; f.Kind != want {
		return &ValidationError{Fields: map[string]string{"file_id": "E-book memakai PDF, audio memakai MP3/M4A"}}
	}
	return nil
}

func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }

func fileParam(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *id, Valid: true}
}

func (s *Service) inTx(ctx context.Context, fn func(*query.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin library transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(query.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit library transaction: %w", err)
	}
	return nil
}

// Create adds a book (teacher/admin). Published books are visible to members immediately.
func (s *Service) Create(ctx context.Context, userID, orgID uuid.UUID, in Input) (query.LibraryBook, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return query.LibraryBook{}, err
	}
	return s.create(ctx, userID, orgID, uuid.New(), in, false)
}

func (s *Service) create(ctx context.Context, userID, orgID, id uuid.UUID, in Input, sample bool) (query.LibraryBook, error) {
	in, err := validate(in)
	if err != nil {
		return query.LibraryBook{}, err
	}
	var book query.LibraryBook
	err = s.inTx(ctx, func(q *query.Queries) error {
		if err := checkFile(ctx, q, orgID, in); err != nil {
			return err
		}
		now := s.now()
		published := pgtype.Timestamptz{}
		if in.Published {
			published = pgTime(now)
		}
		book, err = q.CreateBook(ctx, query.CreateBookParams{
			ID: id, OrgID: orgID, Title: in.Title, Author: in.Author, Shelf: in.Shelf, Level: in.Level,
			Format: in.Format, Copies: int32(in.Copies), FileID: fileParam(in.FileID), Cover: in.Cover,
			Sample: sample, CreatedBy: userID, CreatedAt: pgTime(now), PublishedAt: published,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			// Same ID already exists (idempotent seed): return it unchanged.
			book, err = q.GetBook(ctx, query.GetBookParams{OrgID: orgID, ID: id})
		}
		if err != nil {
			return fmt.Errorf("create book: %w", err)
		}
		return nil
	})
	return book, err
}

// Update replaces all editable fields (teacher/admin). Publishing keeps the first publish time.
func (s *Service) Update(ctx context.Context, userID, orgID, id uuid.UUID, in Input) (query.LibraryBook, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return query.LibraryBook{}, err
	}
	in, err := validate(in)
	if err != nil {
		return query.LibraryBook{}, err
	}
	var book query.LibraryBook
	err = s.inTx(ctx, func(q *query.Queries) error {
		cur, err := q.LockBook(ctx, query.LockBookParams{OrgID: orgID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock book: %w", err)
		}
		if err := checkFile(ctx, q, orgID, in); err != nil {
			return err
		}
		if err := checkCopies(ctx, q, id, in.Copies); err != nil {
			return err
		}
		published := pgtype.Timestamptz{}
		if in.Published {
			published = cur.PublishedAt
			if !published.Valid {
				published = pgTime(s.now())
			}
		}
		book, err = q.UpdateBook(ctx, query.UpdateBookParams{
			OrgID: orgID, ID: id, Title: in.Title, Author: in.Author, Shelf: in.Shelf, Level: in.Level,
			Format: in.Format, Copies: int32(in.Copies), FileID: fileParam(in.FileID), Cover: in.Cover,
			PublishedAt: published, UpdatedAt: pgTime(s.now()),
		})
		if err != nil {
			return fmt.Errorf("update book: %w", err)
		}
		return nil
	})
	return book, err
}

// Archive hides a book (teacher/admin). The file stays; delete it separately from Kelola.
func (s *Service) Archive(ctx context.Context, userID, orgID, id uuid.UUID) error {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return err
	}
	n, err := query.New(s.pool).ArchiveBook(ctx, query.ArchiveBookParams{OrgID: orgID, ID: id, ArchivedAt: pgTime(s.now())})
	if err != nil {
		return fmt.Errorf("archive book: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns one catalog page. Only staff can ask for drafts; participants see published books.
func (s *Service) List(ctx context.Context, userID, orgID uuid.UUID, f Filter) (Page, error) {
	role, err := s.auth.Authorize(ctx, userID, orgID, org.RoleParticipant)
	if err != nil {
		return Page{}, err
	}
	if err := f.validate(); err != nil {
		return Page{}, err
	}
	limit := f.Limit
	if limit < 1 || limit > maxLimit {
		limit = defaultLimit
	}
	params := query.ListBooksParams{
		OrgID: orgID, UserID: userID, IncludeDrafts: f.Drafts && role.AtLeast(org.RoleTeacher),
		RowLimit: int32(limit + 1), Shelf: optText(f.Shelf), Format: optText(f.Format), Level: optText(f.Level),
		Pattern: f.pattern(), Published: pgtype.Bool{Bool: f.Status == "published", Valid: f.Status != ""},
	}
	if f.Cursor != "" {
		title, id, err := decodeCursor(f.Cursor)
		if err != nil {
			return Page{}, ErrInvalidCursor
		}
		params.CursorTitle, params.CursorID = pgtype.Text{String: title, Valid: true}, uuid.NullUUID{UUID: id, Valid: true}
	}
	rows, err := query.New(s.pool).ListBooks(ctx, params)
	if err != nil {
		return Page{}, fmt.Errorf("list books: %w", err)
	}
	page := Page{Items: make([]Book, 0, min(len(rows), limit))}
	for _, r := range rows[:min(len(rows), limit)] {
		b := Book{LibraryBook: r.LibraryBook, Position: r.Position, OnLoan: r.OnLoan}
		if r.MyLoanID.Valid {
			b.MyLoan = &query.LibraryLoan{ID: r.MyLoanID.UUID, BookID: r.LibraryBook.ID, DueAt: r.MyDueAt}
		}
		page.Items = append(page.Items, b)
	}
	if len(rows) > limit {
		last := page.Items[limit-1]
		page.NextCursor = encodeCursor(last.Title, last.ID)
	}
	return page, nil
}

// likeEscape makes user input literal inside ILIKE (default escape character is backslash).
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// Cursor is opaque: base64url("<uuid>|<title>"). Title goes last because it may contain "|".
func encodeCursor(title string, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id.String() + "|" + title))
}

func decodeCursor(c string) (string, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return "", uuid.Nil, err
	}
	rawID, title, ok := strings.Cut(string(b), "|")
	id, err := uuid.Parse(rawID)
	if !ok || err != nil {
		return "", uuid.Nil, ErrInvalidCursor
	}
	return title, id, nil
}

// Uploader stores a sample file through the normal resumable path (quota, checksum, format checks).
type Uploader interface {
	UploadSample(ctx context.Context, userID, orgID, id uuid.UUID, name, kind string, data []byte) error
}

// SeedSample adds one labelled, published sample e-book for an organization, as its first admin.
// IDs derive from the organization, so running it again changes nothing.
func (s *Service) SeedSample(ctx context.Context, files Uploader, orgID uuid.UUID, pdf []byte) (query.LibraryBook, error) {
	admin, err := query.New(s.pool).OrgAdmin(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return query.LibraryBook{}, errors.New("organization not found or has no admin")
	}
	if err != nil {
		return query.LibraryBook{}, fmt.Errorf("find org admin: %w", err)
	}
	fileID := uuid.NewSHA1(orgID, []byte("kelasverse-sample-file-tata-surya"))
	if err := files.UploadSample(ctx, admin, orgID, fileID, "Contoh - Petualangan di Tata Surya.pdf", "pdf", pdf); err != nil {
		return query.LibraryBook{}, fmt.Errorf("upload sample file: %w", err)
	}
	return s.create(ctx, admin, orgID, uuid.NewSHA1(orgID, []byte("kelasverse-sample-book-tata-surya")), Input{
		Title: "Contoh: Petualangan di Tata Surya", Author: "Tim Kelasverse", Shelf: "sains", Level: "SD",
		Format: "ebook", FileID: &fileID, Cover: "ipa", Published: true,
	}, true)
}
