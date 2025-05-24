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

func main() {
	var configPath string
	var showInfo bool
	flag.StringVar(&configPath, "config", "", "configuration file path")
	flag.BoolVar(&showInfo, "info", false, "show app information")
	flag.Parse()
	if showInfo {
		fmt.Println("APP: " + "go-chat")
		fmt.Println("GitHub: " + "https://github.com/farnese17/go-chat.git")
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
		addr := service.Config().Common().ManagerAddress()
		port := service.Config().Common().ManagerPort()
		fmt.Println("Manager API: " + addr + ":" + port)
		if err := managerRouter.Run(fmt.Sprintf("%s:%s", addr, port)); err != nil {
			service.Logger().Fatal("Failed to load router", zap.Error(err))
		}
	}()

	r := router.SetupRouter("release")
	addr := service.Config().Common().HttpAddress()
	port := service.Config().Common().HttpPort()
	fmt.Println("HTTP API: " + addr + ":" + port)
	err := r.Run(fmt.Sprintf("%s:%s", addr, port))
	if err != nil {
		service.Logger().Fatal("Failed to load router", zap.Error(err))
	}
}
