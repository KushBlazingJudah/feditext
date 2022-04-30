package database

import (
	"context"
	"time"
)

// PostID is the type of number used for posts.
// FChannel uses random strings for this, we use numbers.
type PostID uint64

// ModerationActionType is an enum for moderation actions.
type ModerationActionType uint8

// InitFunc is a function signature to make it easier to use any arbitrary
// database.
// Those who wish to implement a new database should create a new file in this
// package, give it a build tag, and provide an InitFunc by placing itself into
// the Engines map in the init function.
type InitFunc func(arg string) (Database, error)

const (
	ModActionBan ModerationActionType = iota
	ModActionWarn
	ModActionDelete
)

var Engines = map[string]InitFunc{}

// Post contains data related to a single post.
// If this is a thread opening post, ID will be equal to Thread.
// ID does not have to be filled out; it will be done while saving to the
// database. It exists purely for the frontend.
type Post struct {
	Thread PostID
	ID     PostID

	Name     string
	Tripcode string

	Date    time.Time
	Content string

	Source string
}

// ModerationAction records any moderation action taken.
// This is used for transparency.
type ModerationAction struct {
	Author string
	Action ModerationActionType
	Board  string
	Post   PostID
	Reason string

	Time time.Time
}

type Board struct {
	ID, Title, Description string
}

// Database implements everything you might need in a textboard database.
// This should be generic enough to port to whatever engine you may like.
type Database interface {
	// Board gets data about a board.
	Board(ctx context.Context, board string) (Board, error)

	// Thread fetches all posts on a thread.
	Thread(ctx context.Context, board string, thread PostID) ([]Post, error)

	// Post fetches a single post from a thread.
	Post(ctx context.Context, board string, post PostID) (Post, error)

	// SaveBoard updates data about a board, or creates a new one.
	SaveBoard(ctx context.Context, board Board) error

	// SavePost saves a post to the database.
	// If Post.ID is 0, one will be generated.
	// If Post.Thread is 0, it is considered a thread.
	SavePost(ctx context.Context, board string, post *Post) error

	// DeleteThread deletes a thread from the database and records a moderation action.
	// It will also delete all posts.
	DeleteThread(ctx context.Context, board string, thread PostID, modAction ModerationAction) error

	// DeletePost deletes a post from the database and records a moderation action.
	DeletePost(ctx context.Context, board string, post PostID, modAction ModerationAction) error

	// Close closes the database. This should only be called upon exit.
	Close() error
}
