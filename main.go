package main

import (
	"context"
	"github.com/filinvadim/dWighter/api"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/handlers"
	"github.com/filinvadim/dWighter/logger"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	echoLog "github.com/labstack/gommon/log"
	middleware "github.com/oapi-codegen/echo-middleware"
	_ "go.uber.org/automaxprocs"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

var version string

const logFormat = `{"time":"${time_rfc3339_nano}","id":"${id}","remote_ip":"${remote_ip}",` +
	`"host":"${host}","method":"${method}","path":"${path}",` +
	`"status":${status},` + "\n"

func main() {
	//m := metrics.NewPrometheusExporter("universal_fix_gateway_errors_counter_total")
	swagger, err := api.GetSwagger()
	if err != nil {
		log.Fatalf("loading swagger spec: %v", err)
	}
	swagger.Servers = nil

	db := storage.New("", "", getAppPath(), true, false, "debug")
	defer db.Close()

	l := logger.NewLogger("info", false)
	l.Infoln("STARTED")

	e := echo.New()

	e.Logger.SetLevel(echoLog.INFO)
	e.Logger.SetPrefix("universal-fix-gateway")

	lg := echomiddleware.LoggerWithConfig(echomiddleware.LoggerConfig{
		Format: logFormat,
	})
	e.Use(lg)
	e.Use(middleware.OapiRequestValidator(swagger))

	api.RegisterHandlers(e, &handlers.Controller{})

	//go func() {
	//	http.Handle("/actuator/"+m.Name(), m.HTTPHandler())
	//	http.ListenAndServe(":8080", nil)
	//}()

	if err = e.Start(net.JoinHostPort("", "6969")); err != nil {
		e.Logger.Error(err)
	}

	e.Shutdown(context.Background())
}

func getAppPath() string {
	var dbPath string

	switch runtime.GOOS {
	case "windows":
		// %LOCALAPPDATA% Windows
		appData := os.Getenv("LOCALAPPDATA") // C:\Users\{username}\AppData\Local
		if appData == "" {
			log.Fatal("failed to get path to LOCALAPPDATA")
		}
		dbPath = filepath.Join(appData, "badgerdb")

	case "darwin", "linux", "android":
		homeDir := os.TempDir()
		dbPath = filepath.Join(homeDir, ".badgerdb")

	default:
		log.Fatal("unsupported OS")
	}

	err := os.MkdirAll(dbPath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	return dbPath
}
