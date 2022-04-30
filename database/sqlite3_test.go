//go:build sqlite3
// +build sqlite3

package database

// TODO. These tests are really awful but get the job done and helped me catch a few mistakes.
// They're also completely generic and have absolutely nothing to do with SQLite3 asides from initTest.

import (
	"context"
	"testing"
)

func initTest() Database {
	db, err := Engines["sqlite3"]("file::memory:?cache=shared")
	if err != nil {
		panic(err)
	}

	return db
}

func makeGarbage(db Database) []PostID {
	posts := []PostID{}

	for i := 0; i < 100; i++ {
		t := Post{
			Name:     "test",
			Tripcode: "abc",
			Content:  "def",
			Source:   "ghi",
		}

		if err := db.SavePost(context.Background(), &t); err != nil {
			panic(err)
		}

		posts = append(posts, t.ID)

		for j := 0; j < 50; j++ {
			p := Post{
				Thread:   t.ID,
				Name:     "test",
				Tripcode: "abc",
				Content:  "hello world",
				Source:   "ghi",
			}

			if err := db.SavePost(context.Background(), &p); err != nil {
				panic(err)
			}
		}
	}

	return posts
}

func TestSqliteDatabase_Thread(t *testing.T) {
	db := initTest()
	defer db.Close()

	ts := makeGarbage(db)

	tests := []struct {
		thread PostID
	}{
		{thread: ts[1]},
		{thread: ts[10]},
		{thread: ts[42]},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, err := db.Thread(context.Background(), tt.thread)
			if err != nil {
				t.Errorf("SqliteDatabase.Thread() error = %v", err)
				return
			}

			if got[26].Content != "hello world" || got[0].Content != "def" {
				t.Errorf("SqliteDatabase.Thread() bad content")
				return
			}
		})
	}
}

func TestSqliteDatabase_Post(t *testing.T) {
	db := initTest()
	defer db.Close()

	ts := makeGarbage(db)

	tests := []struct {
		id PostID
	}{
		{id: ts[1] + 25},
		{id: ts[42] + 16},
		{id: ts[16] + 5},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, err := db.Post(context.Background(), tt.id)
			if err != nil {
				t.Errorf("SqliteDatabase.Post() error = %v", err)
				return
			}

			if got.Content == "" || got.Source == "" || got.Date.IsZero() || got.Tripcode == "" || got.Name == "" {
				t.Errorf("SqliteDatabase.Post() bad return data")
				return
			}
		})
	}
}

func TestSqliteDatabase_SavePost(t *testing.T) {
	db := initTest()
	defer db.Close()

	ts := makeGarbage(db)

	newPost, err := db.Post(context.Background(), ts[1]+16)
	if err != nil {
		panic(err)
	}

	newPost.Content = "new content"
	newPost.Name = "new name"
	newPost.Tripcode = "new trip"
	newPost.Thread = 6969

	tests := []struct {
		post *Post
	}{
		{&newPost},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if err := db.SavePost(context.Background(), tt.post); err != nil {
				t.Errorf("SqliteDatabase.SavePost() error = %v", err)
			}

			retPost, err := db.Post(context.Background(), newPost.ID)
			if err != nil {
				panic(err)
			}

			if retPost.Thread == 6969 {
				t.Errorf("SqliteDatabase.SavePost() changed thread")
				return
			}

			if retPost.Content != newPost.Content || retPost.Name != newPost.Name || retPost.Tripcode != newPost.Tripcode {
				t.Errorf("SqliteDatabase.SavePost() didn't change data")
				return
			}
		})
	}
}

func TestSqliteDatabase_DeleteThread(t *testing.T) {
	db := initTest()
	defer db.Close()

	ts := makeGarbage(db)

	tests := []struct {
		thread    PostID
		modAction ModerationAction
	}{
		{ts[1], ModerationAction{}},
		{ts[2], ModerationAction{}},
		{ts[4], ModerationAction{}},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if err := db.DeleteThread(context.Background(), tt.thread, tt.modAction); err != nil {
				t.Errorf("SqliteDatabase.DeleteThread() error = %v", err)
			}

			_, err := db.Post(context.Background(), tt.thread)
			if err == nil {
				t.Errorf("SqliteDatabase.DeleteThread() didn't delete op")
				return
			}

			_, err = db.Post(context.Background(), tt.thread+16)
			if err == nil {
				t.Errorf("SqliteDatabase.DeleteThread() didn't delete post")
				return
			}
		})
	}
}

func TestSqliteDatabase_DeletePost(t *testing.T) {
	type args struct {
	}
	db := initTest()
	defer db.Close()

	tests := []struct {
		post      PostID
		modAction ModerationAction
	}{
		{14, ModerationAction{}},
		{69, ModerationAction{}},
		{420, ModerationAction{}},
		{1337, ModerationAction{}},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if err := db.DeletePost(context.Background(), tt.post, tt.modAction); err != nil {
				t.Errorf("SqliteDatabase.DeletePost() error = %v", err)
			}

			_, err := db.Post(context.Background(), tt.post)
			if err == nil {
				t.Errorf("SqliteDatabase.DeletePost() didn't delete post")
				return
			}
		})
	}
}
