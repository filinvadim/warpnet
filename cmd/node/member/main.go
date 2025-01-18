package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/logger"
	"github.com/filinvadim/warpnet/server/handlers"
	"github.com/filinvadim/warpnet/server/server"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"

	"syscall"

	frontend "github.com/filinvadim/warpnet-frontend"
	"github.com/filinvadim/warpnet/database/storage"
)

type API struct {
	*handlers.StaticController
	*handlers.AuthController
}

func main() {
	conf, err := config.GetConfig()
	if err != nil {
		log.Fatalf("fail loading config: %v", err)
	}

	fmt.Println("config bootstrap nodes: ", conf.Node.Bootstrap)

	log.Println("Warpnet Version:", warpnet.GetVersion())

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := logger.NewUnifiedLogger(conf.Node.Logging.Level, true)

	db, err := storage.New(getAppPath(), false, conf.Database.DirName, l)
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	nodeRepo := database.NewNodeRepo(db)
	defer nodeRepo.Close()

	authRepo := database.NewAuthRepo(db)
	userRepo := database.NewUserRepo(db)
	//followRepo := database.NewFollowRepo(db)
	//timelineRepo := database.NewTimelineRepo(db)
	//tweetRepo := database.NewTweetRepo(db)
	//replyRepo := database.NewRepliesRepo(db)

	var nodeReadyChan = make(chan string, 1)
	defer close(nodeReadyChan)
	var authReadyChan = make(chan struct{})
	defer close(authReadyChan)

	serverLogger := logger.NewUnifiedLogger(conf.Server.Logging.Level, true)
	interfaceServer, err := server.NewInterfaceServer(conf, serverLogger)
	if err != nil && !errors.Is(err, server.ErrBrowserLoadFailed) {
		log.Fatalf("failed to run public server: %v", err)
	}

	if errors.Is(err, server.ErrBrowserLoadFailed) {
		manualCredsInput(interfaceServer, db)
	}

	userPersistency := struct {
		*database.AuthRepo
		*database.UserRepo
	}{
		authRepo, userRepo,
	}

	authCtrl := handlers.NewAuthController(userPersistency, interruptChan, nodeReadyChan, authReadyChan)
	staticCtrl := handlers.NewStaticController(db.IsFirstRun(), frontend.GetStaticEmbedded())

	interfaceServer.RegisterHandlers(&API{
		staticCtrl,
		authCtrl,
	})
	defer interfaceServer.Shutdown(ctx)
	go interfaceServer.Start()

	select {
	case <-interruptChan:
		log.Println("logged out")
		return
	case <-authReadyChan:
		log.Println("authentication was successful")
	}

	n, err := node.NewRegularNode(
		ctx,
		persistentLayer{nodeRepo, authRepo},
		conf,
		l,
	)
	if err != nil {
		log.Fatalf("failed to init node: %v", err)
	}
	nodeReadyChan <- n.ID()

	defer func() {
		if err := n.Stop(); err != nil {
			log.Fatalf("failed to stop node: %v", err)
		}
	}()

	<-interruptChan
	log.Println("interrupted...")
}

type persistentLayer struct {
	*database.NodeRepo
	*database.AuthRepo
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

	err := os.MkdirAll(dbPath, 0750)
	if err != nil {
		log.Fatal(err)
	}

	return dbPath
}

func manualCredsInput(
	interfaceServer server.PublicServer,
	db *storage.DB,
) {
	if interfaceServer == nil {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter username: ")
		username, _ := reader.ReadString('\n')
		fmt.Print("Enter password: ")
		pass, _ := reader.ReadString('\n')

		if err := db.Run(username, pass); err != nil {
			log.Fatalf("failed to run db: %v", err)
		}
	}
}
