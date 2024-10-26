package main

import (
	"context"
	"fmt"
	"github.com/filinvadim/dWighter/interface/server"
	"github.com/filinvadim/dWighter/interface/server/handlers"
	"github.com/filinvadim/dWighter/node/client"
	"github.com/filinvadim/dWighter/node/node"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

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
	var interruptChan = make(chan os.Signal, 1)

	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := getAppPath()
	db := storage.New(
		path, false,
	)
	defer db.Close()

	interfaceServer, err := server.NewInterfaceServer()
	if err != nil {
		log.Fatalf("failed to run owner server: %v", err)
	}

	cli, err := client.NewNodeClient(ctx)
	if err != nil {
		log.Fatal("node client loading: ", err)
	}

	ip, err := cli.GetOwnIPAddress()
	if err != nil {
		log.Println("failed to get own node ip address")
	}
	fmt.Println("YOUR OWN IP", ip)

	interfaceServer.RegisterHandlers(&API{
		handlers.NewTweetController(cli),
		handlers.NewUserController(cli),
		handlers.NewStaticController(),
		handlers.NewAuthController(cli),
	})
	go interfaceServer.Start()
	defer interfaceServer.Shutdown(ctx)

	n, err := node.NewNodeService(ctx, ip, cli, db, interruptChan)
	if err != nil {
		log.Fatalf("failed to init node service: %v", err)
	}
	defer n.Stop()

	go n.Run()

	<-interruptChan

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
