package server

import (
	"context"
	node_gen "github.com/filinvadim/dWighter/node/node-gen"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	echoLog "github.com/labstack/gommon/log"
	middleware "github.com/oapi-codegen/echo-middleware"
)

const DefaultDiscoveryPort = ":16969"

type DiscoveryServicer interface {
	NewEvent(ctx echo.Context) error
}

type nodeServer struct {
	ctx context.Context
	e   *echo.Echo
}

func NewNodeServer(
	ctx context.Context, service DiscoveryServicer,
) (*nodeServer, error) {
	swagger, err := node_gen.GetSwagger()
	if err != nil {
		return nil, err
	}
	swagger.Servers = nil
	e := echo.New()
	e.HideBanner = true
	e.Logger.SetLevel(echoLog.INFO)
	e.Logger.SetPrefix("node-server")
	e.Use(echomiddleware.Logger())
	//e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.Gzip())
	e.Use(middleware.OapiRequestValidator(swagger))

	node_gen.RegisterHandlers(e, service)

	return &nodeServer{ctx, e}, nil
}

func (ds *nodeServer) Start() error {
	return ds.e.Start(DefaultDiscoveryPort)
}

func (ds *nodeServer) Stop() error {
	return ds.e.Shutdown(ds.ctx)
}
