package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/filinvadim/dWighter/api/api"
	"github.com/filinvadim/dWighter/crypto"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	echoLog "github.com/labstack/gommon/log"
	middleware "github.com/oapi-codegen/echo-middleware"
	"github.com/pkg/browser"
)

type (
	Router            = api.EchoRouter
	HandlersInterface = api.ServerInterface
)

type PublicServerStarter interface {
	Start() error
	Router() Router
	Shutdown(ctx context.Context) error
	RegisterHandlers(publicAPI HandlersInterface)
}

type publicServer struct {
	e    *echo.Echo
	conf *tls.Config
}

const PublicServerHost = "localhost:6969"

func NewPublicServer(path string, logLevel uint8) (PublicServerStarter, error) {
	swagger, err := api.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("loading swagger spec: %v", err)
	}
	swagger.Servers = nil

	e := echo.New()
	e.HideBanner = true
	e.Logger.SetLevel(echoLog.Lvl(logLevel))
	e.Logger.SetPrefix("")

	logFile, err := os.Create(filepath.Join(path, "node.log"))
	if err != nil {
		e.Logger.Fatal(err)
	}
	defer logFile.Close()

	lg := echomiddleware.LoggerWithConfig(echomiddleware.LoggerConfig{
		//Output: logFile,
		Output: os.Stderr,
	})
	e.Use(lg)
	e.Use(echomiddleware.CORS())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.Gzip())
	e.Use(middleware.OapiRequestValidator(swagger))

	conf, err := crypto.GenerateTLSConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to generate TLS config: %v", err)
	}
	conf.ClientAuth = tls.NoClientCert

	return &publicServer{e, conf}, nil
}

func (p *publicServer) Start() error {
	go func() {
		time.Sleep(time.Second)
		err := browser.OpenURL("https://" + PublicServerHost)
		if err != nil {
			p.e.Logger.Errorf("failed to open browser: %v", err)
		}
	}()
	return p.e.StartServer(&http.Server{
		Addr:      PublicServerHost,
		TLSConfig: p.conf,
	})
}

func (p *publicServer) RegisterHandlers(publicAPI HandlersInterface) {
	api.RegisterHandlers(p.e, publicAPI)
}

func (p *publicServer) Router() Router {
	return p.e
}

func (p *publicServer) Shutdown(ctx context.Context) error {
	return p.e.Shutdown(ctx)

}
