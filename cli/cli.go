package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/farnese17/chat/config"
)

var Version string = "dev"

func ParseFlags() string {
	var configPath string
	var showVersion bool
	var generateConfig string
	flag.StringVar(&configPath, "config", "", "configuration file path")
	flag.BoolVar(&showVersion, "version", false, "print version information")
	flag.StringVar(&generateConfig, "generate-config", "", "generate default configuration file")
	flag.Parse()
	if generateConfig != "" {
		path := filepath.Clean(generateConfig)
		if path == "" {
			fmt.Println("please enter a valid directory")
			os.Exit(1)
		}
		cfg := config.GenerateDefaultConfig(path)
		if err := cfg.Save(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("generate default configuration on " + path)
		os.Exit(0)
	}

	if showVersion {
		fmt.Println("Version: " + Version)
		fmt.Println("GitHub: " + "https://github.com/farnese17/go-chat.git")
		os.Exit(0)
	}
	return configPath
}
