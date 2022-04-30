package main

import (
	"github.com/KushBlazingJudah/feditext"
	"github.com/KushBlazingJudah/feditext/config"
)

func main() {
	if err := config.Load("./feditext.config"); err != nil {
		panic(err)
	}

	feditext.Startup()
	defer feditext.Close()

	feditext.Serve()
}
