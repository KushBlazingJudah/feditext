package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/KushBlazingJudah/feditext"
	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
)

var loaded = false

func fatal(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f, a...)
	os.Exit(1)
}

func initdb() {
	var err error

	if config.DatabaseEngine == "" {
		// Preallocate array
		dbs := make([]string, 0, len(database.Engines))

		for k := range database.Engines {
			dbs = append(dbs, k)
		}

		fmt.Printf("No database engine configured.\n")
		fmt.Printf("Available engines: %s\n", strings.Join(dbs, ","))

		os.Exit(1)
	}

	feditext.DB, err = database.Engines[config.DatabaseEngine](config.DatabaseArg)
	if err != nil {
		fatal("Failed initializing database: %v", err)
	}
}

func load(path string) {
	if err := config.Load(path); err != nil && errors.Is(err, os.ErrNotExist) {
		fmt.Printf("Feditext was unable to read it's configuration file, \"%s\"!\n"+
			"You haven't created the file. Please:\n"+
			"- copy doc/config.example to %s\n"+
			"- read its contents carefully and modify as you see fit\n"+
			"- restart Feditext\n", path, path)
		os.Exit(1)
	} else if err != nil {
		fatal("Failed loading config: %v", err)
	}

	// We will have to initialize the database here as everything here requires it.
	initdb()

	loaded = true
}

func createUser(args []string) {
	fls := flag.NewFlagSet(fmt.Sprintf("%s create", os.Args[0]), flag.ExitOnError)

	var (
		cfg      = fls.String("config", "./feditext.config", "location of feditext's config")
		username = fls.String("username", "", "username of new user")
		email    = fls.String("email", "", "email of new user")
		password = fls.String("password", "", "password of new user; read from stdin if not specified")
		priv     = fls.Uint("priv", 0, fmt.Sprintf("privileges of the new user; %d for janitor, %d for moderator, %d for admin.",
			database.ModTypeJanitor, database.ModTypeMod, database.ModTypeAdmin))
	)
	fls.Parse(args)

	load(*cfg)
	defer feditext.DB.Close()

	if *username == "" {
		fmt.Println("Need at least a username to work with. Check out -help.")
		os.Exit(1)
	}

	if *priv > uint(database.ModTypeAdmin) {
		fmt.Printf("-priv was set higher than %d. using %d instead.\n", *priv, uint(database.ModTypeAdmin))
		*priv = uint(database.ModTypeAdmin)
	}

	if *password == "" {
		fmt.Println("Reading password from stdin. Type it, and press enter.\nInput is not hidden.")

		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			panic(scanner.Err())
		}
		*password = scanner.Text()
	}

	if err := feditext.DB.SaveModerator(context.Background(), *username, *email, *password, database.ModType(*priv)); err != nil {
		panic(err)
	}

	os.Exit(0)
}

func opts() {
	switch strings.ToLower(os.Args[1]) {
	case "create": // Create a user.
		createUser(os.Args[2:])
	case "-help":
		fmt.Printf("%s [-config ...]\n", os.Args[0])
		fmt.Printf("%s create -username ... [-password ...] [-priv 0,1,2]\n", os.Args[0])
		// drops to os.Exit(1)
	case "-config":
		if len(os.Args) > 2 {
			load(os.Args[2])
			return // fall back to main
		} else {
			fmt.Println("Specify a configuration file.")
			// drops to os.Exit(1)
		}
	default:
		fmt.Println("Unknown action.")
		fmt.Println("Available are: create.")
		// drops to os.Exit(1)
	}

	os.Exit(1)
}

func main() {
	if len(os.Args) > 1 {
		opts()
	}

	if !loaded {
		load("./feditext.config")
	}

	feditext.Startup()
	defer feditext.Close()

	feditext.Serve()
}
