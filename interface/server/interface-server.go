package server

import (
	"context"
	"fmt"
	"github.com/filinvadim/dWighter/config"
	api_gen "github.com/filinvadim/dWighter/interface/api-gen"
	ownMiddleware "github.com/filinvadim/dWighter/interface/middleware"
	"time"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	middleware "github.com/oapi-codegen/echo-middleware"
	"github.com/pkg/browser"
)

type (
	Router            = api_gen.EchoRouter
	HandlersInterface = api_gen.ServerInterface
)

type PublicServerStarter interface {
	Start()
	Router() Router
	Shutdown(ctx context.Context)
	RegisterHandlers(publicAPI HandlersInterface)
}

type interfaceServer struct {
	e *echo.Echo
}

func NewInterfaceServer() (PublicServerStarter, error) {
	swagger, err := api_gen.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("loading swagger spec: %v", err)
	}
	swagger.Servers = nil

	e := echo.New()
	e.HideBanner = true

	dlc := echomiddleware.DefaultLoggerConfig
	dlc.Format = config.LogFormat
	dlc.Output = e.Logger.Output()

	e.Use(echomiddleware.LoggerWithConfig(dlc))
	e.Use(echomiddleware.CORS())
	e.Use(echomiddleware.Gzip())
	e.Use(middleware.OapiRequestValidator(swagger))
	e.Use(ownMiddleware.NewSessionTokenMiddleware().VerifySessionToken)

	return &interfaceServer{e}, nil
}

func (p *interfaceServer) Start() {
	go func() {
		time.Sleep(time.Second)
		err := browser.OpenURL(config.ExternalNodeAddress.String()) // NOTE connection is not protected!
		if err != nil {
			p.e.Logger.Errorf("failed to open browser: %v", err)
		}
	}()

	if err := p.e.Start(":" + config.ExternalNodeAddress.Port()); err != nil {
		p.e.Logger.Fatal(err)
	}
}

func (p *interfaceServer) RegisterHandlers(publicAPI HandlersInterface) {
	api_gen.RegisterHandlers(p.e, publicAPI)
}

func (p *interfaceServer) Router() Router {
	return p.e
}

func (p *interfaceServer) Shutdown(ctx context.Context) {
	if err := p.e.Shutdown(ctx); err != nil {
		p.e.Logger.Error(err)
	}
}
