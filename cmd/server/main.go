package main

import (
	"log"

	"github.com/ibeloyar/gophprofile/internal/app"
	"github.com/ibeloyar/gophprofile/internal/config"
)

func main() {
	cfg, err := config.ReadConfig()
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
