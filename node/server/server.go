package server

import (
	"context"
	node_gen "github.com/filinvadim/dWighter/node/node-gen"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	middleware "github.com/oapi-codegen/echo-middleware"
	"io/ioutil"
)

const DefaultDiscoveryPort = ":16969"

type NodeServicer interface {
	NewEvent(ctx echo.Context, eventType node_gen.NewEventParamsEventType) error
}

type nodeServer struct {
	ctx context.Context
	e   *echo.Echo
}

func NewNodeServer(
	ctx context.Context, service NodeServicer,
) (*nodeServer, error) {
	swagger, err := node_gen.GetSwagger()
	if err != nil {
		return nil, err
	}
	swagger.Servers = nil
	e := echo.New()
	e.HideBanner = true

	e.Logger.SetOutput(ioutil.Discard)

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
