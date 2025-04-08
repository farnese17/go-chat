package main

import (
	"fmt"

	v1 "github.com/farnese17/chat/api/v1"
	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/repository"
	"github.com/farnese17/chat/router"
	"go.uber.org/zap"
)

func main() {
	service := registry.SetupService()
	defer service.Shutdown()

	if err := repository.Warm(service); err != nil {
		fmt.Println(err)
		return
	}

	go service.Cache().StartFlush()
	service.Cache().BFM().Start()

	if err := v1.SetupFileService(service); err != nil {
		fmt.Println(err)
		return
	}
	v1.SetupUserService(service)
	v1.SetupGroupService(service)
	v1.SetupFriendService(service)
	v1.SetupManagerService(service)

	managerRouter := router.SetupManagerRouter("debug")
	go func() {
		if err := managerRouter.Run(":6000"); err != nil {
			service.Logger().Fatal("Failed to load router", zap.Error(err))
		}
	}()

	r := router.SetupRouter("debug")
	err := r.Run(":3000")
	if err != nil {
		service.Logger().Fatal("Failed to load router", zap.Error(err))
	}
	// logger.Info("Server started!")
}
