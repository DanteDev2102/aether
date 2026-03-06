package main

import (
	"github.com/DanteDev2102/aether/aether"
)

func main() {
	config := &aether.Config{Host: "localhost", Port: 8080, Timeout: 0}

	app := aether.New(config)

	app.Listen()
}
