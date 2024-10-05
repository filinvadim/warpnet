package discovery

import (
	"context"
	"net/http"

	"github.com/filinvadim/dWighter/api/discovery"
	"github.com/filinvadim/dWighter/crypto"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	echoLog "github.com/labstack/gommon/log"
	middleware "github.com/oapi-codegen/echo-middleware"
)

const defaultDiscoveryPort = ":16969"

type DiscoveryServicer interface {
	NewEvent(ctx echo.Context) error
}

type discoveryServer struct {
	ctx context.Context
	e   *echo.Echo
	srv *http.Server
}

func newDiscoveryServer(
	ctx context.Context, service DiscoveryServicer, loggerMw echo.MiddlewareFunc,
) (*discoveryServer, error) {
	swagger, err := discovery.GetSwagger()
	if err != nil {
		return nil, err
	}
	swagger.Servers = nil
	e := echo.New()
	e.Logger.SetLevel(echoLog.INFO)
	e.Logger.SetPrefix("")
	e.Use(loggerMw)
	e.Use(echomiddleware.CORS())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.Gzip())
	e.Use(middleware.OapiRequestValidator(swagger))

	discovery.RegisterHandlers(e, service)

	conf, err := crypto.GenerateTLSConfig("") // TODO just once
	if err != nil {
		return nil, err
	}

	srv := &http.Server{
		Addr:      defaultDiscoveryPort,
		TLSConfig: conf,
	}

	return &discoveryServer{ctx, e, srv}, nil
}

func (ds *discoveryServer) Start() error {
	return ds.e.StartServer(ds.srv)
}

func (ds *discoveryServer) Stop() error {
	return ds.e.Shutdown(ds.ctx)
}
