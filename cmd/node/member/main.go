package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet"
	frontend "github.com/filinvadim/warpnet-frontend"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/server/auth"
	"github.com/filinvadim/warpnet/server/handlers"
	"github.com/filinvadim/warpnet/server/server"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

type API struct {
	*handlers.StaticController
	*handlers.WSController
}

func main() {
	conf, err := config.GetConfig()
	if err != nil {
		log.Fatalf("fail loading config: %v", err)
	}

	version := warpnet.GetVersion()
	log.Println("config bootstrap nodes: ", conf.Node.Bootstrap)
	log.Println("Warpnet Version:", version)

	semVersion, err := semver.NewVersion(strings.TrimSpace(version))
	if err != nil {
		log.Fatalf("fail parsing semantic version: %v %s", err, version)
	}

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, dbCloser, err := storage.New(getAppPath(), false, conf.Database.DirName)
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer dbCloser()

	nodeRepo, closer := database.NewNodeRepo(db)
	defer closer()

	authRepo := database.NewAuthRepo(db)
	userRepo := database.NewUserRepo(db)
	//followRepo := database.NewFollowRepo(db)
	timelineRepo := database.NewTimelineRepo(db)
	tweetRepo := database.NewTweetRepo(db)
	//replyRepo := database.NewRepliesRepo(db)

	var (
		nodeReadyChan = make(chan domain.Owner, 1)
		authReadyChan = make(chan domain.Owner)
	)
	defer func() {
		close(nodeReadyChan)
		close(authReadyChan)
	}()

	interfaceServer, err := server.NewInterfaceServer(conf)
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

	authService := auth.NewAuthService(userPersistency, interruptChan, nodeReadyChan, authReadyChan)
	wsCtrl := handlers.NewWSController(conf, authService)
	staticCtrl := handlers.NewStaticController(db.IsFirstRun(), frontend.GetStaticEmbedded())

	interfaceServer.RegisterHandlers(&API{
		staticCtrl,
		wsCtrl,
	})
	defer interfaceServer.Shutdown(ctx)
	go interfaceServer.Start()

	var owner domain.Owner
	select {
	case <-interruptChan:
		log.Println("logged out")
		return
	case owner = <-authReadyChan:
		log.Println("authentication was successful")
	}

	n, err := node.NewRegularNode(
		ctx,
		persistentLayer{nodeRepo, authRepo},
		conf,
		timelineRepo,
		userRepo,
		tweetRepo,
		semVersion,
	)
	if err != nil {
		log.Fatalf("failed to init node: %v", err)
	}

	owner.NodeId = n.ID().String()
	owner.Ipv6 = n.IPv6()
	owner.Ipv4 = n.IPv4()
	nodeReadyChan <- owner

	defer n.Stop()

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
