package main

import (
	"fmt"

	v1 "github.com/farnese17/chat/api/v1"
	"github.com/farnese17/chat/cli"
	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/repository"
	"github.com/farnese17/chat/router"
	"go.uber.org/zap"
)

func main() {
	configPath := cli.ParseFlags()
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
