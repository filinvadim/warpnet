package discovery

import (
	"context"
	"github.com/filinvadim/dWighter/api/discovery"
	"github.com/filinvadim/dWighter/crypto"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	echoLog "github.com/labstack/gommon/log"
	middleware "github.com/oapi-codegen/echo-middleware"
	"net/http"
)

const defaultDiscoveryPort = ":16969"

type DiscoveryServicer interface {
	// Create a new user
	// (POST /v1/discovery/event/new)
	NewEvent(ctx echo.Context) error
}

type discoveryServer struct {
	e   *echo.Echo
	srv *http.Server
}

func newDiscoveryServer(
	ctx context.Context, service DiscoveryServicer, nodeID string, loggerMw echo.MiddlewareFunc,
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

	conf, err := crypto.GenerateTLSConfig(nodeID) // TODO just once
	if err != nil {
		return nil, err
	}

	srv := &http.Server{
		Addr:      defaultDiscoveryPort,
		TLSConfig: conf,
	}

	return &discoveryServer{e, srv}, nil
}

func (ds *discoveryServer) Start() error {
	return ds.e.StartServer(ds.srv)
}
