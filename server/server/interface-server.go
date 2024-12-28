package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	api_gen "github.com/filinvadim/warpnet/server/api-gen"
	ownMiddleware "github.com/filinvadim/warpnet/server/middleware"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	log2 "github.com/labstack/gommon/log"
	"github.com/pkg/browser"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
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

func NewInterfaceServer(conf config.Config) (PublicServer, error) {
	swagger, err := api_gen.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("loading swagger spec: %v", err)
	}
	swagger.Servers = nil

	e := echo.New()
	e.HideBanner = true
	e.Logger.SetLevel(log2.Lvl(logLevelsMap[strings.ToLower(conf.Server.Logging.Level)]))

	dlc := echomiddleware.DefaultLoggerConfig
	dlc.Format = conf.Server.Logging.Format
	//dlc.Output = e.Logger.Output()
	dlc.Output = io.Discard

	e.Use(echomiddleware.LoggerWithConfig(dlc))
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:  []string{"*"}, // TODO
		AllowHeaders:  []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, "X-SESSION-TOKEN"},
		ExposeHeaders: []string{SessionTokenName}, // ВАЖНО: Разрешить фронтенду видеть заголовок
		AllowMethods:  []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
	}))
	e.Use(echomiddleware.Gzip())
	e.Use(ownMiddleware.NewSessionTokenMiddleware().VerifySessionToken)

	port := ":" + strconv.Itoa(conf.Server.Port)
	err = browser.OpenURL("http://" + conf.Server.Host + port) // NOTE connection is not protected!
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
