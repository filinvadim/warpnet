package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	root "github.com/filinvadim/warpnet"
	frontend "github.com/filinvadim/warpnet-frontend"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/node/client"
	"github.com/filinvadim/warpnet/core/node/member"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/security"
	"github.com/filinvadim/warpnet/server/auth"
	"github.com/filinvadim/warpnet/server/handlers"
	"github.com/filinvadim/warpnet/server/server"
	ipfslog "github.com/ipfs/go-log/v2"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

type API struct {
	*handlers.StaticController
	*handlers.WSController
}

func main() {
	log.Infoln("config bootstrap nodes: ", config.ConfigFile.Node.Bootstrap)
	log.Infoln("Warpnet version:", config.ConfigFile.Version)

	var _ = ipfslog.LevelInfo
	//ipfslog.SetDebugLogging()
	psk, err := security.GeneratePSK(root.GetCodeBase(), config.ConfigFile.Version)
	if err != nil {
		log.Fatal(err)
	}

	// TODO remove
	fmt.Println("GENERATED PSK:", psk.String())

	lvl, err := log.ParseLevel(config.ConfigFile.Logging.Level)
	if err != nil {
		lvl = log.InfoLevel
	}
	log.SetLevel(lvl)

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := storage.New(getAppPath(), false, config.ConfigFile.Database.DirName)
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	authRepo := database.NewAuthRepo(db)
	userRepo := database.NewUserRepo(db)

	var (
		nodeReadyChan = make(chan domain.AuthNodeInfo, 1)
		authReadyChan = make(chan domain.AuthNodeInfo)
	)
	defer func() {
		close(nodeReadyChan)
		close(authReadyChan)
	}()

	interfaceServer, err := server.NewInterfaceServer()
	if err != nil && !errors.Is(err, server.ErrBrowserLoadFailed) {
		log.Fatalf("failed to run public server: %v", err)
	}

	if errors.Is(err, server.ErrBrowserLoadFailed) {
		manualCredsInput(interfaceServer, db)
	}

	clientNode, err := client.NewClientNode(ctx, psk)
	if err != nil {
		log.Fatalf("failed to init client node: %v", err)
	}

	authService := auth.NewAuthService(authRepo, userRepo, interruptChan, authReadyChan)
	wsCtrl := handlers.NewWSController(authService, clientNode)
	staticCtrl := handlers.NewStaticController(db.IsFirstRun(), frontend.GetStaticEmbedded())

	interfaceServer.RegisterHandlers(&API{
		staticCtrl,
		wsCtrl,
	})
	defer interfaceServer.Shutdown(ctx)

	go interfaceServer.Start()

	var serverNodeAuthInfo domain.AuthNodeInfo
	select {
	case <-interruptChan:
		log.Infoln("logged out")
		return
	case serverNodeAuthInfo = <-authReadyChan:
		log.Infoln("authentication was successful")
	}

	serverNode, err := member.NewMemberNode(
		ctx,
		authRepo.PrivateKey().(warpnet.WarpPrivateKey),
		psk,
		authRepo,
		db,
	)
	if err != nil {
		log.Fatalf("failed to init node: %v", err)
	}
	defer serverNode.Stop()

	err = serverNode.Start(clientNode)
	if err != nil {
		log.Fatalf("failed to start member node: %v", err)
	}

	serverNodeAuthInfo.Identity.Owner.NodeId = serverNode.NodeInfo().ID.String()
	serverNodeAuthInfo.NodeInfo = serverNode.NodeInfo()

	nodeReadyChan <- serverNodeAuthInfo

	if err := clientNode.Pair(serverNodeAuthInfo); err != nil {
		log.Fatalf("failed to init client node: %v", err)
	}
	defer clientNode.Stop()

	log.Infoln("WARPNET STARTED")
	<-interruptChan
	log.Infoln("interrupted...")
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
