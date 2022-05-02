package database

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha512"
	"errors"
	"time"
)

// PostID is the type of number used for posts.
// FChannel uses random strings for this, we use numbers.
type PostID uint64

// ModerationActionType is an enum for moderation actions.
type ModerationActionType uint8

// ModType is an enum for moderator types
type ModType uint8

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

const (
	ModTypeJanitor ModType = iota
	ModTypeMod
	ModTypeAdmin
)

const (
	saltLength = 16
)

var (
	ErrPostContents = errors.New("invalid post contents")
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
	Subject  string

	Date     time.Time
	Bumpdate time.Time // Set to above zero to bump if making a new post
	Content  string

	Source string
}

// ModerationAction records any moderation action taken.
// This is used for transparency.
type ModerationAction struct {
	Author string
	Type   ModerationActionType
	Board  string
	Post   PostID
	Reason string

	Date time.Time
}

type Board struct {
	ID, Title, Description string
}

type Report struct {
	ID int

	Source string
	Board  string
	Post   PostID
	Reason string
	Date   time.Time

	Resolved bool
}

type News struct {
	ID int

	Author  string
	Subject string
	Content string

	Date time.Time
}

type Moderator struct {
	Username  string
	Privilege ModType
}

type Ban struct {
	Target  string
	Reason  string
	Date    time.Time
	Expires time.Time
}

// Database implements everything you might need in a textboard database.
// This should be generic enough to port to whatever engine you may like.
type Database interface {
	// Board gets data about a board.
	Board(ctx context.Context, board string) (Board, error)

	// Boards returns a list of all boards.
	Boards(ctx context.Context) ([]Board, error)

	// Threads fetches all threads on a board.
	Threads(ctx context.Context, board string) ([]Post, error)

	// Thread fetches all posts on a thread.
	Thread(ctx context.Context, board string, thread PostID, tail int) ([]Post, error)

	// ThreadStat returns the number of posts and unique posters in any given thread.
	ThreadStat(ctx context.Context, board string, thread PostID) (int, int, error)

	// Post fetches a single post from a thread.
	Post(ctx context.Context, board string, post PostID) (Post, error)

	// Privilege returns the type of moderator username is.
	Privilege(ctx context.Context, username string) (ModType, error)

	// Reports returns a list of reports.
	Reports(ctx context.Context, withResolved bool) ([]Report, error)

	// Audits returns a list of moderator actions.
	Audits(ctx context.Context) ([]ModerationAction, error)

	// News returns news. That's good news.
	News(ctx context.Context) ([]News, error)

	// Moderators returns a list of currently registered moderators.
	Moderators(ctx context.Context) ([]Moderator, error)

	// Captchas returns captcha IDs.
	Captchas(ctx context.Context) ([]string, error)

	// Captcha returns a captcha.
	Captcha(ctx context.Context, id string) ([]byte, string, error)

	// Banned checks to see if a user is banned.
	Banned(ctx context.Context, source string) (bool, time.Time, string, error)

	// Ban bans a user.
	Ban(ctx context.Context, ban Ban, by string) error

	// SaveBoard updates data about a board, or creates a new one.
	SaveBoard(ctx context.Context, board Board) error

	// SavePost saves a post to the database.
	// If Post.ID is 0, one will be generated.
	// If Post.Thread is 0, it is considered a thread.
	SavePost(ctx context.Context, board string, post *Post) error

	// SaveModerator saves a moderator to the database, or updates an existing entry.
	SaveModerator(ctx context.Context, username string, password string, priv ModType) error

	// SaveNews saves news.
	SaveNews(ctx context.Context, news *News) error

	// SaveCaptcha commits a captcha to the database.
	SaveCaptcha(ctx context.Context, id string, solution string, img []byte) error

	// FileReport files a new report for moderators to look at.
	FileReport(ctx context.Context, report Report) error

	// Resolve resolves a report.
	Resolve(ctx context.Context, reportID int) error

	// Solve checks a captcha.
	Solve(ctx context.Context, id, solution string) (bool, error)

	// DeleteThread deletes a thread from the database and records a moderation action.
	// It will also delete all posts.
	DeleteThread(ctx context.Context, board string, thread PostID, modAction ModerationAction) error

	// DeletePost deletes a post from the database and records a moderation action.
	DeletePost(ctx context.Context, board string, post PostID, modAction ModerationAction) error

	// DeleteNews deletes news.
	DeleteNews(ctx context.Context, id int) error

	// DeleteModerator deletes a moderator.
	DeleteModerator(ctx context.Context, username string) error

	// PasswordCheck checks a moderator's password.
	PasswordCheck(ctx context.Context, username string, password string) (bool, error)

	// Close closes the database. This should only be called upon exit.
	Close() error
}

// hash creates a hash of a password with a salt.
func hash(password []byte) ([]byte, []byte) {
	salt := make([]byte, saltLength)
	rand.Read(salt)

	buf := make([]byte, len(password)+len(salt))
	copy(buf, password)
	copy(buf[len(password):], salt)

	hash := sha512.Sum512(buf)
	return hash[:], salt
}

// check checks a password with a salt.
func check(password []byte, salt []byte, target []byte) bool {
	buf := make([]byte, len(password)+len(salt))
	copy(buf, password)
	copy(buf[len(password):], salt)

	hash := sha512.Sum512(buf)
	return bytes.Equal(hash[:], target)
}
