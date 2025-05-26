package cli

import (
	"flag"
	"fmt"
	"os"
)

var Version string = "dev"

func ParseFlags() string {
	var configPath string
	var showVersion bool
	flag.StringVar(&configPath, "config", "", "configuration file path")
	flag.BoolVar(&showVersion, "version", false, "print version information")
	flag.Parse()
	if showVersion {
		fmt.Println("Version: " + Version)
		fmt.Println("GitHub: " + "https://github.com/farnese17/go-chat.git")
		os.Exit(0)
	}
	return configPath
}
