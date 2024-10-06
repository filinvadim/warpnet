package main

import (
	"context"
	"github.com/filinvadim/dWighter/exposed/discovery"
	"github.com/filinvadim/dWighter/local/handlers"
	"github.com/filinvadim/dWighter/local/server"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/database/storage"
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

	srv, err := server.NewOwnerServer(path, 1)
	if err != nil {
		log.Fatalf("failed to run owner server: %v", err)
	}

	srv.RegisterHandlers(&API{
		handlers.NewTweetController(),
		handlers.NewUserController(),
		handlers.NewStaticController(),
		handlers.NewAuthController(interruptChan),
	})
	go func() {
		err := srv.Start()
		if err != nil {
			log.Fatalf("failed to start owner server: %v", err)
		}
	}()

	var discSvc *discovery.DiscoveryService
	for authResult := range interruptChan {
		if authResult.String() == handlers.AuthSuccess {
			discSvc, err = discovery.NewDiscoveryService(
				ctx, nodeRepo, authRepo, userRepo, tweetRepo, timelineRepo, nil,
			)
			if err != nil {
				log.Fatalf("failed to run discovery-gen service: %v", err)
			}
			go discSvc.Run()
		}
	}

	<-interruptChan

	if err = srv.Shutdown(ctx); err != nil {
		log.Printf("failed to shutdown owner server: %v", err)
	}
	if discSvc == nil {
		return
	}
	if err = discSvc.Stop(); err != nil {
		log.Printf("failed to stop discovery service: %v", err)
	}
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
