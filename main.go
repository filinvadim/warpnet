package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/discovery"
	"github.com/filinvadim/dWighter/handlers"
	"github.com/filinvadim/dWighter/server"
	_ "go.uber.org/automaxprocs"
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
	var (
		interruptChan = make(chan os.Signal, 1)
	)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := getAppPath()
	db := storage.New(path, false, "debug")
	defer db.Close()

	nodeRepo := database.NewNodeRepo(db)
	authRepo := database.NewAuthRepo(db)
	followRepo := database.NewFollowRepo(db)
	timelineRepo := database.NewTimelineRepo(db)
	tweetRepo := database.NewTweetRepo(db)
	userRepo := database.NewUserRepo(db)

	srv, err := server.NewPublicServer(path, 1)
	if err != nil {
		log.Fatalf("failed to run public server: %v", err)
	}

	srv.RegisterHandlers(&API{
		handlers.NewTweetController(timelineRepo, tweetRepo),
		handlers.NewUserController(userRepo, followRepo, nodeRepo),
		handlers.NewStaticController(),
		handlers.NewAuthController(authRepo, nodeRepo, interruptChan),
	})
	go srv.Start()

	authResult := <-interruptChan
	if authResult.String() == handlers.AuthSuccess {
		discSvc, err := discovery.NewDiscoveryService(ctx, nodeRepo, authRepo, userRepo, tweetRepo, nil)
		if err != nil {
			log.Fatalf("failed to run discovery service: %v", err)
		}
		go discSvc.Run()
		defer discSvc.Stop()
	}

	<-interruptChan
	srv.Shutdown(ctx)
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
