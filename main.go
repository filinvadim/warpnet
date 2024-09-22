package main

import (
	"context"
	"fmt"
	"github.com/filinvadim/dWighter/api/server"
	"github.com/filinvadim/dWighter/client"
	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/discovery"
	"github.com/filinvadim/dWighter/handlers"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	echoLog "github.com/labstack/gommon/log"
	middleware "github.com/oapi-codegen/echo-middleware"
	"github.com/pkg/browser"
	_ "go.uber.org/automaxprocs"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

var version string

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT)

	swagger, err := server.GetSwagger()
	if err != nil {
		log.Fatalf("loading swagger spec: %v", err)
	}
	swagger.Servers = nil

	path := getAppPath()
	db := storage.New(path, false, "debug")
	defer db.Close()

	nodeRepo := database.NewNodeRepo(db)
	followRepo := database.NewFollowRepo(db)
	timelineRepo := database.NewTimelineRepo(db)
	tweetRepo := database.NewTweetRepo(db)
	userRepo := database.NewUserRepo(db)
	authRepo := database.NewAuthRepo(db)

	e := echo.New()
	e.HideBanner = true
	e.Logger.SetLevel(echoLog.INFO)
	e.Logger.SetPrefix("universal-fix-gateway")

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

	server.RegisterHandlers(e, &struct {
		*handlers.TweetController
		*handlers.NodeController
		*handlers.UserController
		*handlers.StaticController
		*handlers.AuthController
	}{
		handlers.NewTweetController(timelineRepo, tweetRepo),
		handlers.NewNodeController(nodeRepo),
		handlers.NewUserController(userRepo, followRepo, nodeRepo),
		handlers.NewStaticController(),
		handlers.NewAuthController(authRepo, interrupt),
	})

	go e.Start(net.JoinHostPort("localhost", "6969"))

	cli := client.New(context.Background(), e.Logger)
	ds, err := discovery.NewDiscoveryService(cli, nodeRepo, e.Logger)
	if err != nil {
		log.Fatalf("failed to run discovery: %v", err)
	}
	go ds.StartDiscovery()

	err = browser.OpenURL("http://localhost:6969")
	if err != nil {
		fmt.Println("Failed to open browser:", err)
	}

	<-interrupt

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
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
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
