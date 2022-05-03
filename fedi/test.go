//go:build test
// +build test

// Proof of concept file.
// Run if you dare.

package fedi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/KushBlazingJudah/feditext/database"
)

func deserialize() {
	ctx := context.TODO()

	// A bulk of the slowness is the connection speed of the main instance and
	// what I'd have to assume is the sheer amount of time it takes to gather
	// everything up and serialize it.
	res, err := http.Get("https://fchan.xyz/prog/outbox")

	// fp, err := os.Open("./outbox")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	// defer fp.Close()

	dt := time.Now()

	var ob Outbox
	decoder := json.NewDecoder(res.Body)
	// decoder := json.NewDecoder(fp)
	if err := decoder.Decode(&ob); err != nil {
		panic(err)
	}

	// FixOrderedNotes(ob.OrderedNoteCollection, 0)

	df := time.Now()

	// Create a new board if we need to.
	board, err := DB.Board(ctx, ob.Actor.Name)
	if errors.Is(err, sql.ErrNoRows) {
		board = database.Board{ID: ob.Actor.Name, Title: ob.Actor.PreferredUsername, Description: ob.Actor.Summary}
		if err := DB.SaveBoard(ctx, board); err != nil {
			panic(err)
		}
	} else if err != nil {
		panic(err)
	}

	ss := time.Now()

	for i, thread := range ob.OrderedItems {
		fmt.Println(i)

		posts := thread.AsThread()
		op := posts[0]

		// Attempt to save the OP into the database
		// Check if it's already in the database
		if _, err := DB.FindAPID(ctx, board.ID, op.APID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			panic(err)
		} else if !errors.Is(err, sql.ErrNoRows) {
			// Skip! It's already in the database.
			continue
		}

		if err := DB.SavePost(ctx, board.ID, &op); err != nil {
			panic(err)
		}

		for j, post := range posts[1:] {
			fmt.Println(i, j, op.APID, post.APID)

			// Check if it's already in the database
			if _, err := DB.FindAPID(ctx, board.ID, post.APID); err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					panic(err)
				}

			}

			// It's not in the database, we must save it.

			post.Thread = op.ID
			if err := DB.SavePost(ctx, board.ID, &post); err != nil {
				panic(err)
			}
		}
	}

	fmt.Println("deserialization", df.Sub(dt))
	fmt.Println("save into db", time.Since(ss))
}

func serialize() {
	ctx := context.TODO()

	start := time.Now()

	board, err := DB.Board(ctx, "prog")
	if err != nil {
		panic(err)
	}

	outbox, err := GenerateOutbox(ctx, board)
	if err != nil {
		panic(err)
	}

	encoder := json.NewEncoder(io.Discard)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(outbox); err != nil {
		panic(err)
	}

	fmt.Println("serialize", time.Since(start))
}

func _main() {
	var err error
	DB, err = database.Engines["sqlite3"]("./test.db")
	if err != nil {
		panic(err)
	}

	defer DB.Close()

	deserialize()
	serialize()
}
