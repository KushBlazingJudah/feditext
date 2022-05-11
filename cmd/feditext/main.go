package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/KushBlazingJudah/feditext"
	"github.com/KushBlazingJudah/feditext/config"
)

func main() {
	if err := config.Load("./feditext.config"); err != nil && errors.Is(err, os.ErrNotExist) {
		fmt.Printf("Feditext was unable read it's configuration file, ./feditext.config!"+
			"Error: %s\n\n"+
			"Have you created the file? If not:\n"+
			"- copy doc/config.example to ./feditext.config"+
			"- read its contents carefully and modify as you see fit"+
			"- restart Feditext", err)
		os.Exit(1)
	} else if err != nil {
		panic(err)
	}

	feditext.Startup()
	defer feditext.Close()

	feditext.Serve()
}
