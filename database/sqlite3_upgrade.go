//go:build sqlite3
// +build sqlite3

package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/KushBlazingJudah/feditext/util"
)

const sqliteSchema = `
CREATE TABLE boards(
	id TEXT,
	title TEXT,
	description TEXT,

	UNIQUE(id)
);

CREATE TABLE moderators(
	username TEXT,
	hash BLOB,
	salt BLOB,

	type INTEGER,

	UNIQUE(username)
);

CREATE TABLE auditlog(
	id INTEGER PRIMARY KEY ASC,

	type INTEGER,
	date INTEGER,

	author TEXT,
	board TEXT,
	post INTEGER,

	reason INTEGER,

	FOREIGN KEY(board) REFERENCES boards(id),
	FOREIGN KEY(author) REFERENCES moderators(username)
);

CREATE TABLE reports(
	id INTEGER PRIMARY KEY ASC,

	date INTEGER,

	source TEXT,
	board TEXT,
	post INTEGER,

	reason TEXT,

	resolved INTEGER,

	FOREIGN KEY(board) REFERENCES boards(id)
);

CREATE TABLE news(
	id INTEGER PRIMARY KEY ASC,

	date INTEGER,

	author TEXT,
	subject TEXT,
	content TEXT
);

CREATE TABLE captchas(
	id TEXT,
	img BLOB,
	solution TEXT,

	UNIQUE(id)
);

CREATE TABLE bans(
	source TEXT,
	reason TEXT,
	placed INTEGER,
	expires INTEGER,

	UNIQUE(source)
);

CREATE TABLE followers(
	board TEXT,
	source TEXT,

	UNIQUE(board, source),
	FOREIGN KEY(board) REFERENCES boards(id)
);

CREATE TABLE following(
	board TEXT,
	target TEXT,

	UNIQUE(board, target),
	FOREIGN KEY(board) REFERENCES boards(id)
);

CREATE TABLE regexps(
	id INTEGER PRIMARY KEY ASC,
	pattern TEXT,

	UNIQUE(pattern)
);
`

const sqliteNewBoard = `
CREATE TABLE posts_{board}(
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	thread INTEGER,

	name TEXT,
	tripcode TEXT,
	subject TEXT,

	date INTEGER,
	bumpdate INTEGER,

	raw TEXT,
	content TEXT,

	source TEXT,
	apid TEXT,

	flags INTEGER NOT NULL DEFAULT 0,

	UNIQUE(apid)
);

CREATE TABLE replies_{board}(
	id INTEGER PRIMARY KEY AUTOINCREMENT,

	source INTEGER,
	target INTEGER,

	FOREIGN KEY(source) REFERENCES posts_{board}(id),
	FOREIGN KEY(target) REFERENCES posts_{board}(id),
	UNIQUE(source,target)
);
`

var errUpgradeContinue = fmt.Errorf("continue upgrade")

// sqliteUpgrades is a list of functions that upgrade the database's schema
// enough up to the next version after it.
// This is called by quering `PRAGMA user_version` and then selecting all
// functions at in index starting on that number. See SqliteDatabase's init
// function.
//
// XXX: We cannot use the SqliteDatabase struct in here because the functions
// could very well have changed.
// Do *NOT* make assumptions when writing a schema upgrade function.
// Assume that the existing functions do not work.
// Additionally, we use *sql.Tx, so errors can hopefully be just rolled back.
var sqliteUpgrades = []func(*sql.Tx) error{
	func(tx *sql.Tx) error { // New database
		// This is a special case because people could be upgrading from what
		// I'd like to call v0.0.0, or the pre-v0.1.0 days where there was no
		// versioning and just Git commit hashes.
		// In order to prevent manual intervention, we test existence for a
		// table (boards), if it exists then we skip this because the schema
		// has already been initialized.
		//
		// Note: if we start here, this is the only function that will be run
		// because it is assumed that this will bring it to the latest version.
		// A special "errUpgradeContinue" error is used to ignore this fact when
		// migrating from pre-v0.0.0.

		// Check if the table "boards" exists.
		count := 0
		if err := tx.QueryRow(`select count(name) from sqlite_master where type="table" and name="boards"`).Scan(&count); err != nil {
			// This shouldn't fail, something horribly went wrong?
			return err
		}

		if count == 1 {
			// Existing database without PRAGMA user_version
			return errUpgradeContinue
		}

		// We need to initialize a new database
		// This will bring it to the latest version, and no more upgrades will
		// be executed.
		_, err := tx.Exec(sqliteSchema)
		return err
	},
	func(tx *sql.Tx) error { // Flags bitfield
		// Be *extremely* careful here, you cannot simply defer rows.Close() here.

		rows, err := tx.Query(`select id from boards`)
		if err != nil {
			return err
		}

		// Collect a list of boards.
		boards := []string{}
		for rows.Next() {
			board := ""
			if err := rows.Scan(&board); err != nil {
				rows.Close()
				return err
			}
			boards = append(boards, board)
		}
		rows.Close()

		// Modify tables for each board.
		for _, board := range boards {
			// Add the flags field
			if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE posts_%s ADD COLUMN flags INTEGER NOT NULL DEFAULT 0", board)); err != nil {
				return err
			}

			// Update the posts
			rows, err = tx.Query(fmt.Sprintf("SELECT id, date, bumpdate, raw FROM posts_%s", board))
			if err != nil {
				return err
			}

			stmts := []string{}
			for rows.Next() {
				var id int
				var date int
				var bd int
				var raw string
				if err := rows.Scan(&id, &date, &bd, &raw); err != nil {
					rows.Close()
					return err
				}

				flags := 0
				if bd == 0 {
					// I have no idea what I was thinking then
					flags |= flagSage
				}

				if bd == 0 || bd == 1 {
					// Fix this
					stmts = append(stmts, fmt.Sprintf("UPDATE posts_%s SET bumpdate = %d WHERE id = %d", board, date, id))
				}

				if util.IsJapanese(raw) {
					flags |= flagSJIS
				}

				if flags != 0 {
					stmts = append(stmts, fmt.Sprintf("UPDATE posts_%s SET flags = %d WHERE id = %d", board, flags, id))
				}
			}
			rows.Close()

			// We now must run all of our statements
			for _, stmt := range stmts {
				if _, err := tx.Exec(stmt); err != nil {
					return err
				}
			}
		}

		// We should be fine if we made it here.
		return nil
	},
}

// sqliteUpgrade upgrades the SQLite3 database to the latest schema version.
func sqliteUpgrade(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check to see what version we're on
	ver := 0
	if err := tx.QueryRow("PRAGMA user_version").Scan(&ver); err != nil {
		return err
	}

	if ver == len(sqliteUpgrades) {
		// Nothing needs to be done
		return tx.Commit()
	}

	// Save for information
	oldver := ver

	// Special case; possibly uninitialized database
	if ver == 0 {
		if err := sqliteUpgrades[0](tx); err == nil {
			// Database intialized successfully, in sync with sqliteUpgrades
			ver = len(sqliteUpgrades)
			goto done
		} else if errors.Is(err, errUpgradeContinue) {
			// Existing pre-v0.1.0 database
			// We must upgrade all of the way
			ver = 1
		} else if err != nil {
			// Something bad happened
			return err
		}
	}

	// Upgrade to the latest
	for _, fn := range sqliteUpgrades[ver:] {
		if err := fn(tx); err != nil {
			return err
		}
	}
	ver = len(sqliteUpgrades)

done:
	// Set user_version and exit
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", ver)); err != nil {
		return err
	}

	// Everything was successful, continue on!
	log.Printf("Upgraded database from schema version %d to %d", oldver, ver)
	return tx.Commit()
}
