package main

import (
	"flag"
	"fmt"
	"os"

	v1 "github.com/farnese17/chat/api/v1"
	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/repository"
	"github.com/farnese17/chat/router"
	"go.uber.org/zap"
)

var Version string = "dev"

func main() {
	var showVersion bool
	var configPath string
	flag.BoolVar(&showVersion, "version", false, "show version information")
	flag.StringVar(&configPath, "config", "", "configuration file path")
	flag.Parse()
	if showVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	service := registry.SetupService(configPath)
	defer service.Shutdown()

	if err := repository.Warm(service); err != nil {
		fmt.Println(err)
		return
	}

	go service.Cache().StartFlush()
	service.Cache().BFM().Start()

	v1.SetupUserService(service)
	v1.SetupGroupService(service)
	v1.SetupFriendService(service)
	v1.SetupManagerService(service)

	managerRouter := router.SetupManagerRouter("release")
	go func() {
		if err := managerRouter.Run(":6000"); err != nil {
			service.Logger().Fatal("Failed to load router", zap.Error(err))
		}
	}()

	r := router.SetupRouter("release")
	err := r.Run(":3000")
	if err != nil {
		service.Logger().Fatal("Failed to load router", zap.Error(err))
	}
}
