//go:build sqlite3
// +build sqlite3

package database

import (
	"context"
)

type SqliteDatabase struct {
}

func init() {
	Engines["sqlite3"] = func(arg string) (Database, error) {
		return &SqliteDatabase{}, nil
	}
}

// Thread fetches all posts on a thread.
func (db *SqliteDatabase) Thread(ctx context.Context, thread PostID) ([]Post, error) {
	panic("not implemented") // TODO: Implement
}

// Post fetches a single post from a thread.
func (db *SqliteDatabase) Post(ctx context.Context, thread PostID, post PostID) (Post, error) {
	panic("not implemented") // TODO: Implement
}

// SavePost saves a post to the database.
// It must have at least Post.Thread filled out.
func (db *SqliteDatabase) SavePost(ctx context.Context, post Post) error {
	panic("not implemented") // TODO: Implement
}

// DeleteThread deletes a thread from the database and records a moderation action.
// It will also delete all posts.
func (db *SqliteDatabase) DeleteThread(ctx context.Context, thread PostID, modAction ModerationAction) error {
	panic("not implemented") // TODO: Implement
}

// DeletePost deletes a post from the database and records a moderation action.
func (db *SqliteDatabase) DeletePost(ctx context.Context, thread PostID, post PostID, modAction ModerationAction) error {
	panic("not implemented") // TODO: Implement
}

// Close closes the database. This should only be called upon exit.
func (db *SqliteDatabase) Close() error {
	panic("not implemented") // TODO: Implement
}
