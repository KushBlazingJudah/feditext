//go:build sqlite3
// +build sqlite3

package database

import (
	"context"
	"fmt"
	"time"

	"database/sql"

	_ "embed"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sqlite3
var sqliteSchema string

const sqliteNewBoard = `CREATE TABLE IF NOT EXISTS posts_%s(
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	thread INTEGER,

	name TEXT,
	tripcode TEXT,
	date INTEGER,
	options TEXT,
	content TEXT,

	source TEXT
);`

type SqliteDatabase struct {
	conn *sql.DB
}

func init() {
	Engines["sqlite3"] = func(arg string) (Database, error) {
		db, err := sql.Open("sqlite3", arg)
		if err != nil {
			return nil, err
		}

		// Run initial schema
		_, err = db.Exec(sqliteSchema)
		if err != nil {
			db.Close()
			return nil, err
		}

		return &SqliteDatabase{conn: db}, nil
	}
}

func (db *SqliteDatabase) audit(ctx context.Context, modAction ModerationAction) error {
	if modAction.Time.IsZero() {
		modAction.Time = time.Now()
	}

	_, err := db.conn.ExecContext(ctx,
		"INSERT INTO auditlog(type, date, author, board, post, reason) VALUES (?, ?, ?, ?, ?, ?)",
		modAction.Action, modAction.Time, modAction.Author, modAction.Board, modAction.Post, modAction.Reason)
	return err
}

// Board gets data about a board.
func (db *SqliteDatabase) Board(ctx context.Context, id string) (Board, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT title, description FROM boards WHERE id = ?`, id)
	board := Board{ID: id}
	return board, row.Scan(&board.Title, &board.Description)
}

// Boards returns a list of all boards.
func (db *SqliteDatabase) Boards(ctx context.Context) ([]Board, error) {
	rows, err := db.conn.QueryContext(ctx, "SELECT id, title, description FROM boards")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	boards := []Board{}

	for rows.Next() {
		board := Board{}

		if err := rows.Scan(&board.ID, &board.Title, &board.Description); err != nil {
			return boards, err
		}

		boards = append(boards, board)
	}

	return boards, rows.Err()
}

// Threads fetches all threads on a board.
func (db *SqliteDatabase) Threads(ctx context.Context, board string) ([]Post, error) {
	rows, err := db.conn.QueryContext(ctx, fmt.Sprintf(`SELECT id, name, tripcode, date, content, source FROM posts_%s WHERE thread IS 0 ORDER BY id ASC`, board))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []Post{}

	for rows.Next() {
		post := Post{}
		var ttime int64

		if err := rows.Scan(&post.ID, &post.Name, &post.Tripcode, &ttime, &post.Content, &post.Source); err != nil {
			return posts, err
		}

		post.Date = time.Unix(ttime, 0)
		post.Thread = post.ID
		posts = append(posts, post)
	}

	return posts, rows.Err()
}

// Thread fetches all posts on a thread.
func (db *SqliteDatabase) Thread(ctx context.Context, board string, thread PostID) ([]Post, error) {
	rows, err := db.conn.QueryContext(ctx, fmt.Sprintf(`SELECT id, name, tripcode, date, content, source FROM posts_%s WHERE thread IS ? OR id IS ? ORDER BY id ASC`, board), thread, thread)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []Post{}

	for rows.Next() {
		post := Post{Thread: thread}
		var ttime int64

		if err := rows.Scan(&post.ID, &post.Name, &post.Tripcode, &ttime, &post.Content, &post.Source); err != nil {
			return posts, err
		}

		post.Date = time.Unix(ttime, 0)
		posts = append(posts, post)
	}

	// Say no rows if we get nothing back
	err = rows.Err()
	if err == nil && len(posts) == 0 {
		err = sql.ErrNoRows
	}

	return posts, err
}

// Post fetches a single post from a thread.
func (db *SqliteDatabase) Post(ctx context.Context, board string, id PostID) (Post, error) {
	row := db.conn.QueryRowContext(ctx, fmt.Sprintf(`SELECT thread, name, tripcode, date, content, source FROM posts_%s WHERE id = ?`, board), id)
	post := Post{ID: id}

	var ttime int64

	err := row.Scan(&post.Thread, &post.Name, &post.Tripcode, &ttime, &post.Content, &post.Source)
	post.Date = time.Unix(ttime, 0)

	return post, err
}

// Privilege returns the type of moderator username is.
func (db *SqliteDatabase) Privilege(ctx context.Context, username string) (ModType, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT type FROM moderators WHERE username = ?`, username)
	var mt uint8
	return ModType(mt), row.Scan(&mt)
}

// SaveBoard updates data about a board, or creates a new one.
// TODO: We don't actually update a board. Just make a new one.
func (db *SqliteDatabase) SaveBoard(ctx context.Context, board Board) error {
	// This is used to prevent passing an absurdly large amount of arguments.
	// Of course, we still do that, this just looks nicer :)
	args := []interface{}{
		sql.Named("id", board.ID),
		sql.Named("title", board.Title),
		sql.Named("description", board.Description),
	}

	_, err := db.conn.ExecContext(ctx, `INSERT INTO boards(id, title, description) VALUES(:id, :title, :description)`, args...)
	if err != nil {
		return err
	}

	// Create posts table
	_, err = db.conn.ExecContext(ctx, fmt.Sprintf(sqliteNewBoard, board.ID))
	return err
}

// SavePost saves a post to the database.
// If Post.ID is 0, one will be generated. If not, it will update an existing post.
// If Post.Thread is 0, it is considered a thread.
func (db *SqliteDatabase) SavePost(ctx context.Context, board string, post *Post) error {
	if post.Date.IsZero() {
		post.Date = time.Now()
	}

	// This is used to prevent passing an absurdly large amount of arguments.
	// Of course, we still do that, this just looks nicer :)
	args := []interface{}{
		sql.Named("content", post.Content),
		sql.Named("date", post.Date.Unix()),
		sql.Named("name", post.Name),
		sql.Named("source", post.Source),
		sql.Named("thread", post.Thread),
		sql.Named("tripcode", post.Tripcode),
	}

	if post.ID == 0 {
		// We are creating a new post.

		r, err := db.conn.ExecContext(ctx, fmt.Sprintf(`INSERT INTO
			posts_%s(thread, name, tripcode, date, content, source) VALUES (
			:thread, :name, :tripcode, :date, :content, :source)`,
			board), args...)
		if err != nil {
			return err
		}

		id, err := r.LastInsertId()
		post.ID = PostID(id)

		return err
	}

	// We are updating a post if we make it here.
	// We don't update all values of these posts, mostly only the ones that
	// the user controls.
	args = append(args, sql.Named("id", post.ID))
	_, err := db.conn.ExecContext(ctx, fmt.Sprintf(`UPDATE posts_%s SET name =
		:name, tripcode = :tripcode, content = :content WHERE id = :id`,
		board), args...)
	return err
}

// SaveModerator updates data about a moderator, or creates a new one.
// TODO: We don't actually update a moderator. Just make a new one.
func (db *SqliteDatabase) SaveModerator(ctx context.Context, username, password string, priv ModType) error {
	hash, salt := hash([]byte(password))

	// This is used to prevent passing an absurdly large amount of arguments.
	// Of course, we still do that, this just looks nicer :)
	args := []interface{}{
		sql.Named("username", username),
		sql.Named("hash", hash),
		sql.Named("salt", salt),
		sql.Named("type", priv),
	}

	_, err := db.conn.ExecContext(ctx, `INSERT INTO moderators(username, hash, salt, type) VALUES(:username, :hash, :salt, :type)`, args...)
	return err
}

// DeleteThread deletes a thread from the database and records a moderation action.
// It will also delete all posts.
func (db *SqliteDatabase) DeleteThread(ctx context.Context, board string, thread PostID, modAction ModerationAction) error {
	_, err := db.conn.ExecContext(ctx, fmt.Sprintf("DELETE FROM posts_%s WHERE id = ? OR thread = ?", board), thread, thread)
	if err != nil {
		return err
	}

	return db.audit(ctx, modAction)
}

// DeletePost deletes a post from the database and records a moderation action.
func (db *SqliteDatabase) DeletePost(ctx context.Context, board string, post PostID, modAction ModerationAction) error {
	_, err := db.conn.ExecContext(ctx, fmt.Sprintf("DELETE FROM posts_%s WHERE id = ?", board), post)
	if err != nil {
		return err
	}

	return db.audit(ctx, modAction)
}

func (db *SqliteDatabase) password(ctx context.Context, username string) ([]byte, []byte, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT hash, salt FROM moderators WHERE username = ?`, username)

	var hash []byte
	var salt []byte
	return hash, salt, row.Scan(&hash, &salt)
}

// PasswordCheck checks a moderator's password.
func (db *SqliteDatabase) PasswordCheck(ctx context.Context, username string, password string) (bool, error) {
	hash, salt, err := db.password(ctx, username)
	if err != nil {
		return false, err
	}

	return check([]byte(password), salt, hash), nil
}

// Close closes the database. This should only be called upon exit.
func (db *SqliteDatabase) Close() error {
	return db.conn.Close()
}
