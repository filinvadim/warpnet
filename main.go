package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/interface/server"
	"github.com/filinvadim/warpnet/interface/server/handlers"
	client "github.com/filinvadim/warpnet/node-client"
	"github.com/filinvadim/warpnet/node/node"
	"io"
	"log"
	"net/http"
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
	*handlers.TweetController
	*handlers.UserController
	*handlers.StaticController
	*handlers.AuthController
	*handlers.SettingsController
	*handlers.ReplyController
	*handlers.ChatController
}

func main() {
	hosts := flag.String("hosts", "", "comma-separated list of hostnames")
	flag.Parse()

	var predefinedHosts []string
	if hosts != nil && *hosts != "" {
		predefinedHosts = strings.Split(strings.TrimSpace(*hosts), ",")
	}
	fmt.Println("PREDEFINED HOSTS ", predefinedHosts, len(predefinedHosts))

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

	ip, err := GetOwnIPAddress()
	if err != nil {
		log.Println("failed to get own node ip address")
	}
	fmt.Println("YOUR OWN IP", ip)

	interfaceServer.RegisterHandlers(&API{
		handlers.NewTweetController(cli),
		handlers.NewUserController(cli),
		handlers.NewStaticController(staticFolder),
		handlers.NewAuthController(cli),
		handlers.NewSettingsController(cli),
		handlers.NewReplyController(cli),
		handlers.NewChatController(cli),
	})
	go interfaceServer.Start()
	defer interfaceServer.Shutdown(ctx)

	n, err := node.NewNodeService(ctx, ip, predefinedHosts, db, interruptChan)
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

func GetOwnIPAddress() (string, error) {
	for _, addr := range config.IPProviders {
		resp, err := http.Get(addr)
		if err != nil {
			continue
		}

		bt, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		return string(bt), nil
	}
	return "", errors.New("no IP address found")
}
