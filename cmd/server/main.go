package main

import (
	"log"

	"github.com/ibeloyar/gophprofile/internal/app"
	"github.com/ibeloyar/gophprofile/internal/config"
)

func main() {
	if err := app.Run(config.MustReadConfig()); err != nil {
		log.Fatal(err)
	}
}
