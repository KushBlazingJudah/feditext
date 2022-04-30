//go:build sqlite3
// +build sqlite3

package database

import (
	"context"
	"time"

	"database/sql"

	_ "embed"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sqlite3
var sqliteSchema string

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
		"INSERT INTO auditlog(type, date, author, post, reason) VALUES (?, ?, ?, ?, ?)",
		modAction.Action, modAction.Time, modAction.Author, modAction.On, modAction.Reason)
	return err
}

// Thread fetches all posts on a thread.
func (db *SqliteDatabase) Thread(ctx context.Context, thread PostID) ([]Post, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id, name, tripcode, date, content, source FROM posts WHERE id = ? OR thread = ?`, thread, thread)
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

	return posts, rows.Err()
}

// Post fetches a single post from a thread.
func (db *SqliteDatabase) Post(ctx context.Context, id PostID) (Post, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT thread, name, tripcode, date, content, source FROM posts WHERE id = ?`, id)
	post := Post{ID: id}

	var ttime int64

	err := row.Scan(&post.Thread, &post.Name, &post.Tripcode, &ttime, &post.Content, &post.Source)
	post.Date = time.Unix(ttime, 0)

	return post, err
}

// SavePost saves a post to the database.
// If Post.ID is 0, one will be generated. If not, it will update an existing post.
// If Post.Thread is 0, it is considered a thread.
func (db *SqliteDatabase) SavePost(ctx context.Context, post *Post) error {
	if post.Date.IsZero() {
		post.Date = time.Now()
	}

	// This is used to prevent passing an absurdly large amount of arguments.
	// Of course, we still do that, this just looks nicer :)
	args := []interface{}{
		sql.Named("thread", post.Thread),
		sql.Named("name", post.Name),
		sql.Named("tripcode", post.Tripcode),
		sql.Named("date", post.Date.Unix()),
		sql.Named("content", post.Content),
		sql.Named("source", post.Source),
	}

	if post.ID == 0 {
		// We are creating a new post.

		r, err := db.conn.ExecContext(ctx, `INSERT INTO posts(thread, name,
			tripcode, date, content, source) VALUES (:thread, :name,
			:tripcode, :date, :content, :source)`, args...)
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
	_, err := db.conn.ExecContext(ctx, `UPDATE posts SET name = :name,
		tripcode = :tripcode, content = :content WHERE id = :id`,
		args...)
	return err
}

// DeleteThread deletes a thread from the database and records a moderation action.
// It will also delete all posts.
func (db *SqliteDatabase) DeleteThread(ctx context.Context, thread PostID, modAction ModerationAction) error {
	_, err := db.conn.ExecContext(ctx, "DELETE FROM posts WHERE thread = ? OR id = ?", thread, thread)
	if err != nil {
		return err
	}

	return db.audit(ctx, modAction)
}

// DeletePost deletes a post from the database and records a moderation action.
func (db *SqliteDatabase) DeletePost(ctx context.Context, post PostID, modAction ModerationAction) error {
	_, err := db.conn.ExecContext(ctx, "DELETE FROM posts WHERE id = ?", post)
	if err != nil {
		return err
	}

	return db.audit(ctx, modAction)
}

// Close closes the database. This should only be called upon exit.
func (db *SqliteDatabase) Close() error {
	return db.conn.Close()
}
