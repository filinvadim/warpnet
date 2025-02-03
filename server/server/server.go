package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	api_gen "github.com/filinvadim/warpnet/gen/api-gen"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/pkg/browser"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

var (
	ErrBrowserLoadFailed = errors.New("browser load failed")
)

var logLevelsMap = map[string]uint8{
	"debug": 1,
	"info":  2,
	"warn":  3,
	"error": 4,
	"off":   5,
}

type (
	Router            = api_gen.EchoRouter
	HandlersInterface = api_gen.ServerInterface
)

type PublicServer interface {
	Start()
	Router() Router
	Shutdown(ctx context.Context)
	RegisterHandlers(publicAPI HandlersInterface)
}

type interfaceServer struct {
	e    *echo.Echo
	port string
}

func NewInterfaceServer(conf config.Config) (PublicServer, error) {
	swagger, err := api_gen.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("loading swagger spec: %v", err)
	}
	swagger.Servers = nil
	swagger.Paths.Map()

	e := echo.New()
	e.HideBanner = true

	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"*"}, // TODO
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
		AllowMethods: []string{http.MethodGet},
	}))
	e.Use(echomiddleware.Gzip())

	port := ":" + strconv.Itoa(conf.Server.Port)
	err = browser.OpenURL("http://localhost" + port)
	if err != nil {
		log.Errorf("failed to open browser: %v", err)
		return nil, ErrBrowserLoadFailed
	}
	return &interfaceServer{e, port}, nil
}

func (p *interfaceServer) Start() {
	log.Infoln("starting public server...")
	if err := p.e.Start(p.port); err != nil {
		log.Errorf("interface server start: %v", err)
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
	log.Infoln("shutting down public server...")
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
