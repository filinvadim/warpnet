package main

import (
	"context"
	"fmt"
	"github.com/filinvadim/dWighter/api/api"
	"github.com/filinvadim/dWighter/api/components"
	"github.com/filinvadim/dWighter/client"
	"github.com/filinvadim/dWighter/crypto"
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
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

var (
	version string
)

type API struct {
	*handlers.TweetController
	*handlers.UserController
	*handlers.StaticController
	*handlers.AuthController
}

func main() {
	interrupt := make(chan struct{}, 1)

	swagger, err := components.GetSwagger()
	if err != nil {
		log.Fatalf("loading swagger spec: %v", err)
	}
	swagger.Servers = nil

	path := getAppPath()
	db := storage.New(path, false, "debug")
	defer db.Close()

	nodeRepo := database.NewNodeRepo(db)
	authRepo := database.NewAuthRepo(db)

	e := echo.New()
	//e.HideBanner = true
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

	o, err := authRepo.GetOwner()
	if err != nil {
		log.Fatal("main: failed to get owner:", err)
	}
	n, err := nodeRepo.GetByUserId(*o.UserId)
	if err != nil || n == nil {
		log.Fatal("main: failed to get owner node:", err)
	}

	followRepo := database.NewFollowRepo(db)
	timelineRepo := database.NewTimelineRepo(db)
	tweetRepo := database.NewTweetRepo(db)
	userRepo := database.NewUserRepo(db)

	api.RegisterHandlers(e, &API{
		handlers.NewTweetController(timelineRepo, tweetRepo),
		handlers.NewUserController(userRepo, followRepo, nodeRepo),
		handlers.NewStaticController(),
		handlers.NewAuthController(authRepo, nodeRepo, interrupt),
	})
	conf, err := crypto.GenerateTLSConfig(n.Id.String())
	if err != nil {
		log.Fatalf("failed to generate TLS config: %v", err)
	}

	srv := &http.Server{
		Addr:      ":6969",
		TLSConfig: conf,
	}
	go e.StartServer(srv)

	err = browser.OpenURL("https://localhost:6969")
	if err != nil {
		fmt.Println("failed to open browser:", err)
	}

	cli, err := client.New(context.Background(), n.Id.String(), e.Logger)
	if err != nil {
		log.Fatalf("failed to run client: %v", err)
	}
	ds, err := discovery.NewDiscoveryService(cli, nodeRepo, e.Logger)
	if err != nil {
		log.Fatalf("failed to run discovery: %v", err)
	}
	go ds.StartDiscovery()

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
