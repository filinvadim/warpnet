package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	api_gen "github.com/filinvadim/warpnet/interface/api-gen"
	ownMiddleware "github.com/filinvadim/warpnet/interface/middleware"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/pkg/browser"
	"net/http"
)

var ErrBrowserLoadFailed = errors.New("browser load failed")

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
	//dlc.Output = io.Discard

	e.Use(echomiddleware.LoggerWithConfig(dlc))
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:  []string{"*"}, // TODO
		AllowHeaders:  []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, "X-SESSION-TOKEN"},
		ExposeHeaders: []string{"X-SESSION-TOKEN"}, // ВАЖНО: Разрешить фронтенду видеть заголовок
		AllowMethods:  []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
	}))
	e.Use(echomiddleware.Gzip())
	e.Use(ownMiddleware.NewSessionTokenMiddleware().VerifySessionToken)

	err = browser.OpenURL(config.ExternalNodeAddress.String()) // NOTE connection is not protected!
	if err != nil {
		e.Logger.Errorf("failed to open browser: %v", err)
		return nil, ErrBrowserLoadFailed
	}
	return &interfaceServer{e}, nil
}

func (p *interfaceServer) Start() {
	if err := p.e.Start(":" + config.ExternalNodeAddress.Port()); err != nil {
		p.e.Logger.Fatalf("interface server start: %v", err)
	}
}

func (p *interfaceServer) RegisterHandlers(publicAPI HandlersInterface) {
	api_gen.RegisterHandlers(p.e, publicAPI)
}

func (p *interfaceServer) Router() Router {
	return p.e
}

func (p *interfaceServer) Shutdown(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			p.e.Logger.Error(r)
		}
	}()
	if p == nil {
		return
	}
	if p.e == nil {
		return
	}
	if err := p.e.Shutdown(ctx); err != nil {
		p.e.Logger.Error(err)
	}
}
