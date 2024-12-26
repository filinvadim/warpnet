package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/filinvadim/warpnet/core/node"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/interface/server"
	"github.com/filinvadim/warpnet/interface/server/handlers"
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
	version string
)

//go:embed interface/static
var staticFolder embed.FS

type API struct {
	*handlers.StaticController
	*handlers.AuthController
}

func main() {
	var (
		hosts       = new(string)
		isBootstrap = new(bool)
	)
	hosts = flag.String("hosts", "", "comma-separated list of hostnames")
	isBootstrap = flag.Bool("is-bootstrap", false, "is bootstrap node?")
	flag.Parse()

	var bootstrapAddrs []string
	if *hosts != "" {
		bootstrapAddrs = strings.Split(strings.TrimSpace(*hosts), ",")
	}
	fmt.Println("IS BOOTSTRAP:", *isBootstrap)
	fmt.Println("BOOTSTRAP ADDRESSES", bootstrapAddrs, len(bootstrapAddrs))

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := getAppPath(*isBootstrap)
	db := storage.New(
		path, *isBootstrap,
	)
	defer db.Close()

	nodeRepo := database.NewNodeRepo(db)
	authRepo := database.NewAuthRepo(db)
	userRepo := database.NewUserRepo(db)

	var nodeReadyChan = make(chan string, 1)
	defer close(nodeReadyChan)
	var authReadyChan = make(chan struct{})
	defer close(authReadyChan)

	if *isBootstrap {
		if err := db.Run("bootstrap", "bootstrap"); err != nil {
			log.Fatalln("DB bootstrap run failed", err)
		}
	}
	if !*isBootstrap {
		//followRepo := database.NewFollowRepo(db)
		//timelineRepo := database.NewTimelineRepo(db)
		//tweetRepo := database.NewTweetRepo(db)
		//replyRepo := database.NewRepliesRepo(db)

		interfaceServer, err := server.NewInterfaceServer()
		if err != nil && !errors.Is(err, server.ErrBrowserLoadFailed) {
			log.Fatalf("failed to run owner handler: %v", err)
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
			return
		case <-authReadyChan:
			log.Println("authentication was successful")
		}
	}

	n, err := node.NewNode(
		ctx,
		nodeRepo,
		authRepo,
		*isBootstrap,
	)
	if err != nil {
		log.Fatalf("failed to init node: %v", err)
	}
	nodeReadyChan <- n.ID()
	log.Println("NODE STARTED WITH ID", n.ID())
	log.Println("AND ADDRESSES", n.Addresses())

	defer func() {
		if err := n.Stop(); err != nil {
			log.Fatalf("failed to stop node: %v", err)
		}
	}()

	<-interruptChan
	fmt.Println("interrupted...")
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
	interfaceServer server.PublicServerStarter,
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
