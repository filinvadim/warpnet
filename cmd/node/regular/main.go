package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/server/server"
	"github.com/filinvadim/warpnet/server/server/handlers"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"

	"syscall"

	"embed"
	"github.com/filinvadim/warpnet/database/storage"
	_ "go.uber.org/automaxprocs"
)

var (
	version = "0.0.1"
)

//go:embed static
var staticFolder embed.FS

//go:embed config.yml
var configFile []byte

type API struct {
	*handlers.StaticController
	*handlers.AuthController
}

func main() {
	var conf config.Config
	if err := yaml.Unmarshal(configFile, &conf); err != nil {
		log.Fatal("unmarshalling config: ", err)
	}

	fmt.Println("config bootstrap nodes: ", conf.Node.BootstrapAddrs)

	version = conf.Version.String()

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := storage.New(getAppPath(), false, conf.Database.Dir)
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

	interfaceServer.RegisterHandlers(&API{
		handlers.NewStaticController(staticFolder),
		handlers.NewAuthController(
			userPersistency, interruptChan, nodeReadyChan, authReadyChan),
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

	err := os.MkdirAll(dbPath, os.ModePerm)
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
