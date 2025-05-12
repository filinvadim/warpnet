// Copyright 2025 Vadim Filil
// SPDX-License-Identifier: gpl

package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/server/api-gen"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/pkg/browser"
	log "github.com/sirupsen/logrus"
	"net/http"
)

var (
	ErrBrowserLoadFailed = errors.New("browser load failed")
)

type (
	Router            = api.EchoRouter
	HandlersInterface = api.ServerInterface
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

func NewInterfaceServer() (PublicServer, error) {
	conf := config.ConfigFile
	swagger, err := api.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("loading swagger spec: %v", err)
	}
	swagger.Servers = nil
	swagger.Paths.Map()

	e := echo.New()
	e.HideBanner = true

	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{"localhost"}, // TODO
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
		AllowMethods: []string{http.MethodGet},
	}))
	e.Use(echomiddleware.Gzip())
	e.Use(secureHeadersMiddleware)

	port := ":" + conf.Server.Port
	err = browser.OpenURL("http://localhost" + port)
	if err != nil {
		log.Errorf("failed to open browser: %v", err)
		return nil, ErrBrowserLoadFailed
	}
	return &interfaceServer{e, port}, nil
}

func (p *interfaceServer) Start() {
	log.Infoln("starting public server...")
	err := p.e.Start(p.port)
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			log.Infoln("public server stopped")
			return
		}
		log.Errorf("interface server start: %v", err)
	}
}

func (p *interfaceServer) RegisterHandlers(publicAPI HandlersInterface) {
	api.RegisterHandlers(p.e, publicAPI)
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

func secureHeadersMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		if err := next(ctx); err != nil {
			ctx.Error(err)
		}
		ctx.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
		ctx.Response().Header().Set("Pragma", "no-cache")
		ctx.Response().Header().Set("Expires", "0")
		ctx.Response().Header().Set("X-Content-Type-Options", "nosniff")
		ctx.Response().Header().Set("X-Frame-Options", "deny")
		ctx.Response().Header().Set("Content-Security-Policy", "default-src 'none'")
		ctx.Response().Header().Set("X-Powered-By", "")
		ctx.Response().Header().Set("Server", "")
		ctx.Response().Header().Set("Content-Type", "application/json")
		return nil
	}
}
