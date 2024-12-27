package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/filinvadim/warpnet/core/node"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/server/server"
	"github.com/filinvadim/warpnet/server/server/handlers"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"embed"
	"github.com/filinvadim/warpnet/database/storage"
	_ "go.uber.org/automaxprocs"
)

var (
	version = "0.0.1"
)

//go:embed server/static
var staticFolder embed.FS

type API struct {
	*handlers.StaticController
	*handlers.AuthController
}

func main() {
	hosts := flag.String("bootstrap-hosts", "", "comma-separated list of hostnames")
	isBstp := flag.Bool("is-bootstrap", false, "is bootstrap node?")
	flag.Parse()
	isBootstrap := *isBstp

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	var bootstrapAddrs []string
	if hosts != nil && *hosts != "" {
		bootstrapAddrs = strings.Split(strings.TrimSpace(*hosts), ",")
	}
	log.Println("IS BOOTSTRAP:", isBootstrap)
	log.Println("BOOTSTRAP ADDRESSES", bootstrapAddrs, len(bootstrapAddrs))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := getAppPath(isBootstrap)
	db := storage.New(
		path, isBootstrap,
	)
	defer db.Close()

	nodeRepo := database.NewNodeRepo(db)
	authRepo := database.NewAuthRepo(db)
	userRepo := database.NewUserRepo(db)

	var nodeReadyChan = make(chan string, 1)
	defer close(nodeReadyChan)
	var authReadyChan = make(chan struct{})
	defer close(authReadyChan)

	if !isBootstrap {
		interfaceServer, err := server.NewInterfaceServer()
		if err != nil && !errors.Is(err, server.ErrBrowserLoadFailed) {
			log.Fatalf("failed to run public server: %v", err)
		}
		defer interfaceServer.Shutdown(ctx)

		if interfaceServer != nil && !errors.Is(err, server.ErrBrowserLoadFailed) {
			interfaceServer.RegisterHandlers(&API{
				handlers.NewStaticController(staticFolder),
				handlers.NewAuthController(
					authRepo, userRepo, interruptChan, nodeReadyChan, authReadyChan),
			})
			go interfaceServer.Start()
		} else {
			manualCredsInput(interfaceServer, db)
		}

		select {
		case <-interruptChan:
			log.Println("logged out")
			return
		case <-authReadyChan:
			log.Println("authentication was successful")
		}
	}

	n, err := node.NewNode(
		ctx,
		nodeRepo,
		authRepo,
		isBootstrap,
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

func getAppPath(isBootstrap bool) string {
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

	if isBootstrap {
		dbPath = dbPath + "/bootstrap"
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

//followRepo := database.NewFollowRepo(db)
//timelineRepo := database.NewTimelineRepo(db)
//tweetRepo := database.NewTweetRepo(db)
//replyRepo := database.NewRepliesRepo(db)
