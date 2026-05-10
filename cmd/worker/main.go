package main

import (
	"log"

	"github.com/ibeloyar/gophprofile/internal/config"
	"github.com/ibeloyar/gophprofile/internal/worker"
)

func main() {
	if err := worker.Run(config.MustReadConfig()); err != nil {
		log.Fatal(err)
	}
}
