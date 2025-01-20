package main

import (
	"fmt"
	"os"

	"github.com/OrtemRepos/shortlink/configs"
	"github.com/OrtemRepos/shortlink/internal/app"
)

var cfg *configs.Config

func initConfig() {
	cfgInit, err := configs.GetConfig(os.Args[1:])
	if err != nil {
		fmt.Println(err)
	}
	cfg = cfgInit
}

func main() {
	initConfig()
	app.Run(cfg)
}
