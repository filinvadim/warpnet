package server

import (
	"context"
	node_gen "github.com/filinvadim/dWighter/node/node-gen"
	"net/http"

	"github.com/filinvadim/dWighter/crypto"
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
	srv *http.Server
}

func NewNodeServer(
	ctx context.Context, service DiscoveryServicer, loggerMw echo.MiddlewareFunc,
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
	e.Use(loggerMw)
	e.Use(echomiddleware.CORS())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.Gzip())
	e.Use(middleware.OapiRequestValidator(swagger))

	node_gen.RegisterHandlers(e, service)

	conf, err := crypto.GenerateTLSConfig("") // TODO just once
	if err != nil {
		return nil, err
	}

	srv := &http.Server{
		Addr:      DefaultDiscoveryPort,
		TLSConfig: conf,
	}

	return &nodeServer{ctx, e, srv}, nil
}

func (ds *nodeServer) Start() error {
	return ds.e.StartServer(ds.srv)
}

func (ds *nodeServer) Stop() error {
	return ds.e.Shutdown(ds.ctx)
}
