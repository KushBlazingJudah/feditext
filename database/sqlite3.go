//go:build sqlite3
// +build sqlite3

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/KushBlazingJudah/feditext/config"
	_ "github.com/mattn/go-sqlite3"

	"math/rand"
)

type SqliteDatabase struct {
	conn    *sql.DB
	regexps map[int]*regexp.Regexp
}

func init() {
	Engines["sqlite3"] = func(arg string) (Database, error) {
		db, err := sql.Open("sqlite3", arg+"?cache=shared")
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(1)

		// Run initial schema
		if err := sqliteUpgrade(db); err != nil {
			db.Close()
			return nil, fmt.Errorf("upgrade database: %w", err)
		}

		sdb := &SqliteDatabase{conn: db, regexps: make(map[int]*regexp.Regexp)}

		// Fetch regexps and compile them
		regexps, err := sdb.Regexps(context.Background())
		if err != nil {
			return sdb, fmt.Errorf("compiling regexps: %w", err)
		}

		for _, rexp := range regexps {
			re, err := regexp.Compile(rexp.Pattern)
			if err != nil {
				log.Printf("failed to compile regexp %d: %s", rexp.ID, rexp.Pattern)
				continue
			}

			sdb.regexps[rexp.ID] = re
		}

		return sdb, nil
	}
}

func (db *SqliteDatabase) audit(ctx context.Context, modAction ModerationAction) error {
	if modAction.Date.IsZero() {
		modAction.Date = time.Now().UTC()
	}

	_, err := db.conn.ExecContext(ctx,
		"INSERT INTO auditlog(type, date, author, board, post, reason) VALUES (?, ?, ?, ?, ?, ?)",
		modAction.Type, modAction.Date.Unix(), modAction.Author, modAction.Board, modAction.Post, modAction.Reason)
	return err
}

// Board gets data about a board.
func (db *SqliteDatabase) Board(ctx context.Context, id string) (Board, error) {
	board := Board{}

	if err := db.conn.QueryRowContext(ctx, `SELECT id, title, description FROM boards WHERE id = ?`, id).Scan(&board.ID, &board.Title, &board.Description); err != nil {
		return board, err
	}

	if err := db.conn.QueryRowContext(ctx, fmt.Sprintf(`SELECT count() FROM posts_%s WHERE thread = 0`, id)).Scan(&board.Threads); err != nil {
		return board, err
	}

	return board, nil
}

// Boards returns a list of all boards.
func (db *SqliteDatabase) Boards(ctx context.Context) ([]Board, error) {
	rows, err := db.conn.QueryContext(ctx, "SELECT id, title, description FROM boards ORDER BY id")
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
// TODO: specify sort. We assume that we're just going to sort by latest bumped threads.
// This is true in 99% of cases but not always.
func (db *SqliteDatabase) Threads(ctx context.Context, board string, page int) ([]Post, error) {
	var rows *sql.Rows
	var err error

	if page > 0 {
		offset := page*config.ThreadsPerPage
		limit := config.ThreadsPerPage
		rows, err = db.conn.QueryContext(ctx, fmt.Sprintf(`SELECT id, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags FROM posts_%s WHERE thread IS 0 ORDER BY bumpdate DESC LIMIT ? OFFSET ?`, board), limit, offset)
	} else {
		rows, err = db.conn.QueryContext(ctx, fmt.Sprintf(`SELECT id, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags FROM posts_%s WHERE thread IS 0 ORDER BY bumpdate DESC`, board))
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []Post{}

	for rows.Next() {
		post := Post{}
		var ttime int64
		var btime int64 // Should never be nil
		flags := 0

		if err := rows.Scan(&post.ID, &post.Name, &post.Tripcode, &post.Subject, &ttime, &post.Raw, &post.Content, &post.Source, &btime, &post.APID, &flags); err != nil {
			return posts, err
		}

		post.Date = time.Unix(ttime, 0).UTC()
		post.Bumpdate = time.Unix(btime, 0).UTC()
		post.Thread = post.ID
		post.readFlags(flags)

		posts = append(posts, post)
	}

	return posts, rows.Err()
}

// Thread fetches all posts on a thread.
func (db *SqliteDatabase) Thread(ctx context.Context, board string, thread PostID, tail int, replies bool) ([]Post, error) {
	tx, err := db.conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var rows *sql.Rows
	if tail > 0 {
		rows, err = tx.QueryContext(ctx, fmt.Sprintf(`SELECT id, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags FROM posts_%s WHERE id = :thread OR id IN (SELECT id FROM posts_%s WHERE thread = :thread ORDER BY id DESC LIMIT :tail);`, board, board), sql.Named("thread", thread), sql.Named("tail", tail))
	} else {
		rows, err = tx.QueryContext(ctx, fmt.Sprintf(`SELECT id, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags FROM posts_%s WHERE thread IS ? OR id IS ? ORDER BY id ASC`, board), thread, thread)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []Post{}

	for rows.Next() {
		post := Post{Thread: thread}
		var ttime int64
		var btime *int64 // Will most likely be nil
		flags := 0

		if err := rows.Scan(&post.ID, &post.Name, &post.Tripcode, &post.Subject, &ttime, &post.Raw, &post.Content, &post.Source, &btime, &post.APID, &flags); err != nil {
			return posts, err
		}

		post.Date = time.Unix(ttime, 0).UTC()
		if btime != nil {
			post.Bumpdate = time.Unix(*btime, 0).UTC()
		}
		post.readFlags(flags)

		if replies {
			post.Replies, err = db.repliesTx(ctx, tx, board, post.ID)
			if err != nil {
				return posts, err
			}
		}

		posts = append(posts, post)
	}

	// Say no rows if we get nothing back
	err = rows.Err()
	if err == nil && len(posts) == 0 {
		err = sql.ErrNoRows
	}

	tx.Commit() // TODO: Unsure how to handle this error, should be non-fatal anyway.

	return posts, err
}

// ThreadStat returns the number of posts and unique posters in any given thread.
func (db *SqliteDatabase) ThreadStat(ctx context.Context, board string, thread PostID) (int, int, error) {
	row := db.conn.QueryRowContext(ctx, fmt.Sprintf(`SELECT count(id), count(distinct source) FROM posts_%s WHERE id IS ? OR thread IS ?`, board), thread, thread)

	var posts int
	var posters int

	return posts, posters, row.Scan(&posts, &posters)
}

// Post fetches a single post from a thread.
func (db *SqliteDatabase) Post(ctx context.Context, board string, id PostID) (Post, error) {
	row := db.conn.QueryRowContext(ctx, fmt.Sprintf(`SELECT thread, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags FROM posts_%s WHERE id = ?`, board), id)
	post := Post{ID: id}

	var ttime int64
	var btime *int64
	flags := 0

	err := row.Scan(&post.Thread, &post.Name, &post.Tripcode, &post.Subject, &ttime, &post.Raw, &post.Content, &post.Source, &btime, &post.APID, &flags)
	if err != nil {
		return post, err
	}

	post.Date = time.Unix(ttime, 0).UTC()
	if btime != nil {
		post.Bumpdate = time.Unix(*btime, 0).UTC()
	}
	post.readFlags(flags)

	return post, err
}

// postTx fetches a single post from a thread.
// Keep in sync with Post.
func (db *SqliteDatabase) postTx(ctx context.Context, tx *sql.Tx, board string, id PostID) (Post, error) {
	row := tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT thread, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags FROM posts_%s WHERE id = ?`, board), id)
	post := Post{ID: id}

	var ttime int64
	var btime *int64
	flags := 0

	err := row.Scan(&post.Thread, &post.Name, &post.Tripcode, &post.Subject, &ttime, &post.Raw, &post.Content, &post.Source, &btime, &post.APID, &flags)
	if err != nil {
		return post, err
	}

	post.Date = time.Unix(ttime, 0).UTC()
	if btime != nil {
		post.Bumpdate = time.Unix(*btime, 0).UTC()
	}
	post.readFlags(flags)

	return post, err
}

// FindAPID finds a post given its ActivityPub ID.
func (db *SqliteDatabase) FindAPID(ctx context.Context, board string, apid string) (Post, error) {
	row := db.conn.QueryRowContext(ctx, fmt.Sprintf(`SELECT id, thread, name, tripcode, subject, date, raw, content, source, bumpdate, flags FROM posts_%s WHERE apid = ?`, board), apid)
	post := Post{APID: apid}

	var ttime int64
	var btime *int64
	flags := 0

	err := row.Scan(&post.ID, &post.Thread, &post.Name, &post.Tripcode, &post.Subject, &ttime, &post.Raw, &post.Content, &post.Source, &btime, &flags)
	if err != nil {
		return post, err
	}

	post.Date = time.Unix(ttime, 0).UTC()
	if btime != nil {
		post.Bumpdate = time.Unix(*btime, 0).UTC()
	}
	post.readFlags(flags)

	return post, err
}

// findAPIDTx finds a post given its ActivityPub ID.
// Keep in sync with FindAPID.
func (db *SqliteDatabase) findAPIDTx(ctx context.Context, tx *sql.Tx, board string, apid string) (Post, error) {
	row := tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT id, thread, name, tripcode, subject, date, raw, content, source, bumpdate, flags FROM posts_%s WHERE apid = ?`, board), apid)
	post := Post{APID: apid}

	var ttime int64
	var btime *int64
	flags := 0

	err := row.Scan(&post.ID, &post.Thread, &post.Name, &post.Tripcode, &post.Subject, &ttime, &post.Raw, &post.Content, &post.Source, &btime, &flags)
	if err != nil {
		return post, err
	}

	post.Date = time.Unix(ttime, 0).UTC()
	if btime != nil {
		post.Bumpdate = time.Unix(*btime, 0).UTC()
	}
	post.readFlags(flags)

	return post, err
}

// Privilege returns the type of moderator username is.
func (db *SqliteDatabase) Privilege(ctx context.Context, username string) (ModType, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT type FROM moderators WHERE username = ?`, username)
	var mt uint8
	return ModType(mt), row.Scan(&mt)
}

// Reports returns a list of reports.
func (db *SqliteDatabase) Reports(ctx context.Context, inclResolved bool) ([]Report, error) {
	query := `SELECT id, source, date, board, post, reason, resolved FROM reports`
	if !inclResolved {
		query += ` WHERE resolved IS 0`
	}

	rows, err := db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := []Report{}

	for rows.Next() {
		report := Report{}
		var ttime int64

		if err := rows.Scan(&report.ID, &report.Source, &ttime, &report.Board, &report.Post, &report.Reason, &report.Resolved); err != nil {
			return reports, err
		}

		report.Date = time.Unix(ttime, 0).UTC()
		reports = append(reports, report)
	}

	return reports, rows.Err()
}

// BoardReports returns a list of reports specific to a board.
func (db *SqliteDatabase) BoardReports(ctx context.Context, board string, inclResolved bool) ([]Report, error) {
	query := `SELECT id, source, date, post, reason, resolved FROM reports WHERE board = ?`
	if !inclResolved {
		query += ` AND resolved IS 0`
	}

	rows, err := db.conn.QueryContext(ctx, query, board)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := []Report{}

	for rows.Next() {
		report := Report{Board: board}
		var ttime int64

		if err := rows.Scan(&report.ID, &report.Source, &ttime, &report.Post, &report.Reason, &report.Resolved); err != nil {
			return reports, err
		}

		report.Date = time.Unix(ttime, 0).UTC()
		reports = append(reports, report)
	}

	return reports, rows.Err()
}

// Audits returns a list of moderator actions.
func (db *SqliteDatabase) Audits(ctx context.Context) ([]ModerationAction, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT type, date, author, board, post, reason FROM auditlog ORDER BY id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	acts := []ModerationAction{}

	for rows.Next() {
		act := ModerationAction{}
		var ttime int64

		if err := rows.Scan(&act.Type, &ttime, &act.Author, &act.Board, &act.Post, &act.Reason); err != nil {
			return acts, err
		}

		act.Date = time.Unix(ttime, 0).UTC()
		acts = append(acts, act)
	}

	return acts, rows.Err()
}

// News returns news. That's good news.
func (db *SqliteDatabase) News(ctx context.Context) ([]News, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id, author, subject, content, date FROM news ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	allNews := []News{}

	for rows.Next() {
		news := News{}
		var ttime int64

		if err := rows.Scan(&news.ID, &news.Author, &news.Subject, &news.Content, &ttime); err != nil {
			return allNews, err
		}

		news.Date = time.Unix(ttime, 0).UTC()
		allNews = append(allNews, news)
	}

	return allNews, rows.Err()
}

// Article returns a specific news article.
func (db *SqliteDatabase) Article(ctx context.Context, id int) (*News, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT author, subject, content, date FROM news WHERE id = ?`, id)
	news := News{ID: id}
	var ttime int64

	if err := row.Scan(&news.Author, &news.Subject, &news.Content, &ttime); err != nil {
		return nil, err
	}

	news.Date = time.Unix(ttime, 0).UTC()
	return &news, nil
}

// Moderators returns a list of currently registered moderators.
func (db *SqliteDatabase) Moderators(ctx context.Context) ([]Moderator, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT username, type FROM moderators`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mods := []Moderator{}

	for rows.Next() {
		mod := Moderator{}

		if err := rows.Scan(&mod.Username, &mod.Privilege); err != nil {
			return mods, err
		}

		mods = append(mods, mod)
	}

	return mods, rows.Err()
}

// Captchas returns captcha IDs.
func (db *SqliteDatabase) Captchas(ctx context.Context) ([]string, error) {
	rows, err := db.conn.QueryContext(ctx, "SELECT id FROM captchas")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	caps := []string{}

	for rows.Next() {
		cap := ""

		if err := rows.Scan(&cap); err != nil {
			return caps, err
		}

		caps = append(caps, cap)
	}

	return caps, rows.Err()
}

// Captcha returns a captcha.
func (db *SqliteDatabase) Captcha(ctx context.Context, id string) ([]byte, string, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT img, solution FROM captchas WHERE id = ?`, id)
	img := []byte{}
	sol := ""
	return img, sol, row.Scan(&img, &sol)
}

// repliesTx returns a list of IDs to a post.
func (db *SqliteDatabase) repliesTx(ctx context.Context, tx *sql.Tx, board string, id PostID) ([]Post, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`SELECT id, thread, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags FROM posts_%s WHERE id IN (SELECT source FROM replies_%s WHERE target = ?)`, board, board), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []Post{}

	for rows.Next() {
		post := Post{}
		var ttime int64
		var btime int64 // Should never be nil
		flags := 0

		if err := rows.Scan(&post.ID, &post.Thread, &post.Name, &post.Tripcode, &post.Subject, &ttime, &post.Raw, &post.Content, &post.Source, &btime, &post.APID, &flags); err != nil {
			return posts, err
		}

		post.Date = time.Unix(ttime, 0).UTC()
		post.Bumpdate = time.Unix(btime, 0).UTC()
		post.readFlags(flags)

		posts = append(posts, post)
	}

	return posts, rows.Err()
}

// Replies returns a list of IDs to a post.
func (db *SqliteDatabase) Replies(ctx context.Context, board string, id PostID, reverse bool) ([]Post, error) {
	var rows *sql.Rows
	var err error
	if reverse {
		rows, err = db.conn.QueryContext(ctx, fmt.Sprintf(`SELECT id, thread, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags FROM posts_%s WHERE id IN (SELECT target FROM replies_%s WHERE source = ?)`, board, board), id)
	} else {
		rows, err = db.conn.QueryContext(ctx, fmt.Sprintf(`SELECT id, thread, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags FROM posts_%s WHERE id IN (SELECT source FROM replies_%s WHERE target = ?)`, board, board), id)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []Post{}

	for rows.Next() {
		post := Post{}
		var ttime int64
		var btime int64 // Should never be nil
		flags := 0

		if err := rows.Scan(&post.ID, &post.Thread, &post.Name, &post.Tripcode, &post.Subject, &ttime, &post.Raw, &post.Content, &post.Source, &btime, &post.APID, &flags); err != nil {
			return posts, err
		}

		post.Date = time.Unix(ttime, 0).UTC()
		post.Bumpdate = time.Unix(btime, 0).UTC()
		post.readFlags(flags)

		posts = append(posts, post)
	}

	return posts, rows.Err()
}

// Following returns a list of Actors a board is following.
func (db *SqliteDatabase) Following(ctx context.Context, board string) ([]string, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT target FROM following WHERE board = ?`, board)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	following := []string{}

	for rows.Next() {
		f := ""
		if err := rows.Scan(&f); err != nil {
			return following, err
		}

		following = append(following, f)
	}

	return following, rows.Err()
}

// Followers returns a list of Actors a board is being followed by.
func (db *SqliteDatabase) Followers(ctx context.Context, board string) ([]string, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT source FROM followers WHERE board = ?`, board)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	followers := []string{}

	for rows.Next() {
		f := ""
		if err := rows.Scan(&f); err != nil {
			return followers, err
		}

		followers = append(followers, f)
	}

	return followers, rows.Err()
}

// Regexps returns a list of regular expressions for filtering posts.
func (db *SqliteDatabase) Regexps(ctx context.Context) ([]Regexp, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id,pattern FROM regexps`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	regexps := []Regexp{}

	for rows.Next() {
		f := Regexp{}
		if err := rows.Scan(&f.ID, &f.Pattern); err != nil {
			return regexps, err
		}

		regexps = append(regexps, f)
	}

	return regexps, rows.Err()
}

// Banned checks to see if a user is banned.
func (db *SqliteDatabase) Banned(ctx context.Context, source string) (bool, time.Time, string, error) {
	row := db.conn.QueryRowContext(ctx, "SELECT expires, reason FROM bans WHERE source = ?", source)

	var ttime *int64
	reason := ""

	if err := row.Scan(&ttime, &reason); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, time.Time{}, reason, err
	} else if errors.Is(err, sql.ErrNoRows) {
		// Not banned
		return true, time.Time{}, "", nil
	}

	if ttime != nil {
		exp := time.Unix(*ttime, 0).UTC()

		if time.Now().UTC().After(exp) {
			// Delete
			_, err := db.conn.ExecContext(ctx, "DELETE FROM bans WHERE source = ?", source)
			return true, exp, "", err
		}

		return false, exp, reason, nil
	}

	return false, time.Time{}, reason, nil
}

// AddFollow records an Actor as following a board.
func (db *SqliteDatabase) AddFollow(ctx context.Context, source string, board string) error {
	_, err := db.conn.ExecContext(ctx, "INSERT OR IGNORE INTO followers(source, board) VALUES(?, ?)", source, board)
	return err
}

// AddFollowing records a board is following an Actor.
func (db *SqliteDatabase) AddFollowing(ctx context.Context, board string, target string) error {
	_, err := db.conn.ExecContext(ctx, "INSERT OR IGNORE INTO following(board, target) VALUES(?, ?)", board, target)
	return err
}

// AddRegexp adds a regular expression to the post filter.
func (db *SqliteDatabase) AddRegexp(ctx context.Context, pattern string) error {
	// Compile it first
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	n, err := db.conn.ExecContext(ctx, "INSERT OR IGNORE INTO regexps(pattern) VALUES(?)", pattern)
	if err != nil {
		return err
	}

	var id int64
	if id, err = n.LastInsertId(); err == nil {
		db.regexps[int(id)] = re
	}

	return err
}

// Ban bans a user.
func (db *SqliteDatabase) Ban(ctx context.Context, ban Ban, by string) error {
	// This is used to prevent passing an absurdly large amount of arguments.
	// Of course, we still do that, this just looks nicer :)
	args := []interface{}{
		sql.Named("source", ban.Target),
		sql.Named("placed", time.Now().UTC().Unix()),
		sql.Named("expires", ban.Expires.Unix()),
		sql.Named("reason", ban.Reason),
	}

	_, err := db.conn.ExecContext(ctx, `INSERT INTO bans(source, placed, expires, reason) VALUES(:source, :placed, :expires, :reason) ON CONFLICT(source) DO UPDATE SET expires = excluded.expires, reason = excluded.reason`, args...)
	if err != nil {
		return err
	}

	// Create audit entry
	return db.audit(ctx, ModerationAction{
		Author: by,
		Type:   ModActionBan,
		Reason: fmt.Sprintf("banned %s until %s: %s", ban.Target, ban.Expires.String(), ban.Reason),
		Date:   time.Now().UTC(),
	})
}

// SaveBoard updates data about a board, or creates a new one.
func (db *SqliteDatabase) SaveBoard(ctx context.Context, board Board) error {
	// This is used to prevent passing an absurdly large amount of arguments.
	// Of course, we still do that, this just looks nicer :)
	args := []interface{}{
		sql.Named("id", board.ID),
		sql.Named("title", board.Title),
		sql.Named("description", board.Description),
	}

	_, err := db.conn.ExecContext(ctx, `INSERT INTO boards(id, title, description) VALUES(:id, :title, :description) ON CONFLICT(id) DO UPDATE SET title = excluded.title, description = excluded.description`, args...)
	if err != nil {
		return err
	}

	// Create posts table
	_, err = db.conn.ExecContext(ctx, strings.ReplaceAll(sqliteNewBoard, "{board}", board.ID))
	return err
}

func (db *SqliteDatabase) findPost(ctx context.Context, tx *sql.Tx, board string) func(match string) (Post, error) {
	return func(match string) (Post, error) {
		if match[0] == 'h' { // AP
			return db.findAPIDTx(ctx, tx, board, match)
		}

		id, _ := strconv.Atoi(match) // Won't fail
		return db.postTx(ctx, tx, board, PostID(id))
	}
}

// SavePostTx saves a post to the database, in a transaction.
// If Post.ID is 0, one will be generated. If not, it will update an existing post.
// If Post.Thread is 0, it is considered a thread.
func (db *SqliteDatabase) SavePostTx(ctx context.Context, tx *sql.Tx, board string, post *Post) error {
	if post.Date.IsZero() {
		post.Date = time.Now().UTC()
	}

	// Forbid empty posting
	if strings.TrimSpace(post.Raw) == "" {
		return ErrPostContents
	}

	// Check to see if this post is hit by the filter
	for _, regexp := range db.regexps {
		if regexp.MatchString(post.Raw) {
			return ErrPostRejected
		}
	}

	// Generate APID
	// Random hex number for now
	if post.APID == "" {
		// TODO: This really sucks. Really. However, it does work well enough. 16^8 possible different IDs.
		// This sucks even more to avoid a bug.
		post.APID = fmt.Sprintf("%s://%s/%s/%c%07X", config.TransportProtocol, config.FQDN, board, []byte("ABCDEF")[rand.Intn(6)], rand.Intn(math.MaxInt32)&0xfffffff)
	}

	// Format the post from raw unless we don't need to
	var err error
	var reps []PostID
	if post.Content == "" {
		repmap := findReplies(post)
		reps, err = formatPost(board, post, repmap, db.findPost(ctx, tx, board))
		if err != nil {
			return err
		}

		// We cannot do anything with the replies yet because we don't know the post's ID.
		// And in order to record the reply, we need the post's ID.
	}

	// This is used to prevent passing an absurdly large amount of arguments.
	// Of course, we still do that, this just looks nicer :)
	args := []interface{}{
		sql.Named("apid", post.APID),
		sql.Named("content", post.Content),
		sql.Named("date", post.Date.Unix()),
		sql.Named("name", post.Name),
		sql.Named("raw", post.Raw),
		sql.Named("source", post.Source),
		sql.Named("subject", post.Subject),
		sql.Named("thread", post.Thread),
		sql.Named("tripcode", post.Tripcode),
		sql.Named("bumpdate", post.Date.Unix()),
		sql.Named("flags", post.flags()),
	}

	if post.ID == 0 {
		// We are creating a new post.

		r, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO
			posts_%s(thread, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags) VALUES (
				:thread, :name, :tripcode, :subject, :date, :raw, :content, :source, :bumpdate, :apid, :flags)`,
			board), args...)
		if err != nil {
			return err
		}

		id, err := r.LastInsertId()
		post.ID = PostID(id)

		// Don't mark a thread as replying to a post.
		if post.Thread != 0 {
			// Now, we can place in our replies, long after they were deferred.
			for _, v := range reps {
				if err := db.addReplyTx(ctx, tx, board, post.ID, v); err != nil {
					return err
				}
			}

			if !post.Sage {
				// I used to use post.Date but you could send posts to the
				// bottom of the board that way with a specially crafted
				// activity.
				if _, err := tx.ExecContext(ctx, fmt.Sprintf(`UPDATE posts_%s SET bumpdate = ? WHERE id = ?`, board), time.Now().UTC().Unix(), post.Thread); err != nil {
					return err
				}
			}
		}

		return err
	}

	// We are updating a post if we make it here.
	// We don't update all values of these posts, mostly only the ones that
	// the user controls.
	args = append(args, sql.Named("id", post.ID))
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`UPDATE posts_%s SET name =
		:name, tripcode = :tripcode, subject = :subject, raw = :raw, content = :content WHERE id = :id`,
		board), args...)
	return err
}

// SavePost saves a post to the database.
// If Post.ID is 0, one will be generated. If not, it will update an existing post.
// If Post.Thread is 0, it is considered a thread.
func (db *SqliteDatabase) SavePost(ctx context.Context, board string, post *Post) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := db.SavePostTx(ctx, tx, board, post); err != nil {
		return err
	}

	return tx.Commit()
}

// SaveModerator updates data about a moderator, or creates a new one.
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

	_, err := db.conn.ExecContext(ctx, `INSERT INTO moderators(username, hash, salt, type) VALUES(:username, :hash, :salt, :type) ON CONFLICT(username) DO UPDATE SET hash = excluded.hash, salt = excluded.salt, type = excluded.type`, args...)
	return err
}

// SaveNews saves news.
// If News.ID is 0, a new article is created.
func (db *SqliteDatabase) SaveNews(ctx context.Context, news *News) error {
	if news.Date.IsZero() {
		news.Date = time.Now().UTC()
	}

	// Forbid empty posting
	if strings.TrimSpace(news.Content) == "" {
		return ErrPostContents
	}

	// This is used to prevent passing an absurdly large amount of arguments.
	// Of course, we still do that, this just looks nicer :)
	args := []interface{}{
		sql.Named("author", news.Author),
		sql.Named("subject", news.Subject),
		sql.Named("content", news.Content),
		sql.Named("date", news.Date.UTC().Unix()),
	}

	if news.ID == 0 {
		// We are creating a new article.

		r, err := db.conn.ExecContext(ctx, `INSERT INTO news(author, subject,
			content, date) VALUES (:author, :subject, :content, :date)`,
			args...)
		if err != nil {
			return err
		}

		id, err := r.LastInsertId()
		news.ID = int(id)

		return err
	}

	// We are updating news if we make it here.
	args = append(args, sql.Named("id", news.ID))
	_, err := db.conn.ExecContext(ctx, `UPDATE news SET subject = :subject,
		content = :content WHERE id = :id`, args...)
	return err
}

// SaveCaptcha commits a captcha to the database.
func (db *SqliteDatabase) SaveCaptcha(ctx context.Context, id string, solution string, img []byte) error {
	_, err := db.conn.ExecContext(ctx, `INSERT INTO captchas(id, img, solution) VALUES(?, ?, ?)`, id, img, solution)
	return err
}

// FileReport files a new report for moderators to look at.
func (db *SqliteDatabase) FileReport(ctx context.Context, report Report) error {
	args := []interface{}{
		sql.Named("source", report.Source),
		sql.Named("board", report.Board),
		sql.Named("post", report.Post),
		sql.Named("reason", report.Reason),
		sql.Named("date", time.Now().UTC().Unix()),
	}

	_, err := db.conn.ExecContext(ctx, `INSERT INTO reports(source, board, post, reason, date, resolved) VALUES(:source, :board, :post, :reason, :date, 0)`, args...)
	return err
}

// Resolve resolves a report.
func (db *SqliteDatabase) Resolve(ctx context.Context, id int) error {
	_, err := db.conn.ExecContext(ctx, `UPDATE reports SET resolved = 1 WHERE id = ?`, id)
	return err
}

// Solve checks a captcha.
func (db *SqliteDatabase) Solve(ctx context.Context, id, solution string) (bool, error) {
	solution = strings.ToUpper(solution)

	row := db.conn.QueryRowContext(ctx, `SELECT solution FROM captchas WHERE id = ?`, id)
	sol := ""

	if err := row.Scan(&sol); err != nil {
		return false, err
	}

	_, err := db.conn.ExecContext(ctx, `DELETE FROM captchas WHERE id = ?`, id)
	return solution == sol, err
}

// AddReply links two posts together as a reply.
func (db *SqliteDatabase) AddReply(ctx context.Context, board string, from, to PostID) error {
	_, err := db.conn.ExecContext(ctx, fmt.Sprintf(`INSERT OR IGNORE INTO replies_%s(source, target) VALUES(?, ?)`, board), from, to)
	return err
}

// addReplyTx links two posts together as a reply.
// Keep in sync with AddReply.
func (db *SqliteDatabase) addReplyTx(ctx context.Context, tx *sql.Tx, board string, from, to PostID) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT OR IGNORE INTO replies_%s(source, target) VALUES(?, ?)`, board), from, to)
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

// DeleteNews deletes news.
func (db *SqliteDatabase) DeleteNews(ctx context.Context, id int) error {
	_, err := db.conn.ExecContext(ctx, "DELETE FROM news WHERE id = ?", id)
	return err
}

// DeleteModerator deletes a moderator.
func (db *SqliteDatabase) DeleteModerator(ctx context.Context, username string) error {
	_, err := db.conn.ExecContext(ctx, "DELETE FROM moderators WHERE username = ?", username)
	return err
}

// DeleteFollow removes a follow from the "followers" entry from a board.
func (db *SqliteDatabase) DeleteFollow(ctx context.Context, source string, board string) error {
	_, err := db.conn.ExecContext(ctx, "DELETE FROM followers WHERE source = ? AND board = ?", source, board)
	return err
}

// DeleteFollowing removes a follow from the "following" entry from a board.
func (db *SqliteDatabase) DeleteFollowing(ctx context.Context, board string, target string) error {
	_, err := db.conn.ExecContext(ctx, "DELETE FROM following WHERE board = ? AND target = ?", board, target)
	return err
}

// DeleteRegexp removes a regular expression from the post filter.
func (db *SqliteDatabase) DeleteRegexp(ctx context.Context, id int) error {
	// Remove it from our state
	delete(db.regexps, id)

	_, err := db.conn.ExecContext(ctx, "DELETE FROM regexps WHERE id = ?", id)
	return err
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

func (db *SqliteDatabase) RecentPosts(ctx context.Context, board string, limit int, local bool) ([]Post, error) {
	var rows *sql.Rows
	var err error

	if local {
		rows, err = db.conn.QueryContext(ctx, fmt.Sprintf(`SELECT id, thread, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags FROM posts_%s WHERE source NOT LIKE "http%%" ORDER BY date DESC LIMIT ?`, board), limit)
	} else {
		rows, err = db.conn.QueryContext(ctx, fmt.Sprintf(`SELECT id, thread, name, tripcode, subject, date, raw, content, source, bumpdate, apid, flags FROM posts_%s ORDER BY date DESC LIMIT ?`, board), limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []Post{}

	for rows.Next() {
		post := Post{}
		var ttime int64
		var btime *int64 // Will most likely be nil
		flags := 0

		if err := rows.Scan(&post.ID, &post.Thread, &post.Name, &post.Tripcode, &post.Subject, &ttime, &post.Raw, &post.Content, &post.Source, &btime, &post.APID, &flags); err != nil {
			return posts, err
		}

		post.Date = time.Unix(ttime, 0).UTC()
		if btime != nil {
			post.Bumpdate = time.Unix(*btime, 0).UTC()
		}
		post.readFlags(flags)

		posts = append(posts, post)
	}

	return posts, rows.Err()
}

// Close closes the database. This should only be called upon exit.
func (db *SqliteDatabase) Close() error {
	return db.conn.Close()
}
