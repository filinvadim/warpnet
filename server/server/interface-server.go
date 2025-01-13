package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	api_gen "github.com/filinvadim/warpnet/server/api-gen"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/pkg/browser"
	"log"
	"net/http"
	"strconv"
)

const SessionTokenName = "X-SESSION-TOKEN"

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

func NewInterfaceServer(conf config.Config, l echo.Logger) (PublicServer, error) {
	swagger, err := api_gen.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("loading swagger spec: %v", err)
	}
	swagger.Servers = nil

	e := echo.New()
	e.HideBanner = true
	e.Logger = l

	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:  []string{"*"}, // TODO
		AllowHeaders:  []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, "X-SESSION-TOKEN"},
		ExposeHeaders: []string{SessionTokenName}, // ВАЖНО: Разрешить фронтенду видеть заголовок
		AllowMethods:  []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
	}))
	e.Use(echomiddleware.Gzip())

	l.Warn("VerifySessionToken temp disabled!") // TODO
	//e.Use(ownMiddleware.NewSessionTokenMiddleware().VerifySessionToken)

	port := ":" + strconv.Itoa(conf.Server.Port)
	err = browser.OpenURL("http://localhost" + port) // NOTE connection is not protected!
	if err != nil {
		e.Logger.Errorf("failed to open browser: %v", err)
		return nil, ErrBrowserLoadFailed
	}
	return &interfaceServer{e, port}, nil
}

func (p *interfaceServer) Start() {
	log.Println("starting public server...")
	if err := p.e.Start(p.port); err != nil {
		p.e.Logger.Printf("interface server start: %v", err)
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
	log.Println("shutting down public server...")
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
