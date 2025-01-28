package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	frontend "github.com/filinvadim/warpnet-frontend"
	"github.com/filinvadim/warpnet/config"
	dht "github.com/filinvadim/warpnet/core/dhash-table"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/handler"
	"github.com/filinvadim/warpnet/core/mdns"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/node/member"
	"github.com/filinvadim/warpnet/core/pubsub"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
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

	log.Println("config bootstrap nodes: ", conf.Node.Bootstrap)
	log.Println("Warpnet Version:", conf.Version)

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
		nodeReadyChan = make(chan domain.AuthNodeInfo, 1)
		authReadyChan = make(chan domain.AuthNodeInfo)
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

	var authInfo domain.AuthNodeInfo
	select {
	case <-interruptChan:
		log.Println("logged out")
		return
	case authInfo = <-authReadyChan:
		log.Println("authentication was successful")
	}
	privKey := authRepo.PrivateKey().(warpnet.WarpPrivateKey)
	persLayer := persistentLayer{nodeRepo, authRepo}

	discService := discovery.NewDiscoveryService(ctx, userRepo, persLayer)
	defer discService.Close()
	mdnsService := mdns.NewMulticastDNS(ctx, discService.HandlePeerFound)
	defer mdnsService.Close()
	pubsubService := pubsub.NewPubSub(ctx, discService.HandlePeerFound)
	defer pubsubService.Close()
	providerStore, err := dht.NewProviderCache(ctx, persLayer)
	if err != nil {
		log.Fatalf("failed to init providers: %v", err)
	}
	defer providerStore.Close()

	dHashTable := dht.NewDHTable(
		ctx, persLayer, providerStore, conf,
		discService.HandlePeerFound,
		dht.DefaultNodeRemovedCallback,
	)
	defer dHashTable.Close()

	n, err := member.NewMemberNode(
		ctx,
		privKey,
		persLayer,
		conf,
		dHashTable.StartRouting,
		conf.Version,
	)
	if err != nil {
		log.Fatalf("failed to init node: %v", err)
	}
	defer n.Stop()

	go discService.Run(n)
	go mdnsService.Start(n)
	go pubsubService.RunDiscovery(n)

	authInfo.Identity.Owner.NodeId = n.ID().String()
	authInfo.Identity.Owner.Ipv6 = n.IPv6()
	authInfo.Identity.Owner.Ipv4 = n.IPv4()
	authInfo.Version = conf.Version.String()

	mw := middleware.NewWarpMiddleware()
	n.SetStreamHandler(stream.PairPrivate, handler.StreamNodesPairingHandler(mw, authInfo))
	n.SetStreamHandler(stream.TimelinePrivate, handler.StreamTimelineHandler(mw, timelineRepo))
	n.SetStreamHandler(stream.TweetPrivate, handler.StreamNewTweetHandler(mw, tweetRepo, timelineRepo))

	n.SetStreamHandler(stream.UserPublic, handler.StreamGetUserHandler(mw, userRepo))
	n.SetStreamHandler(stream.TweetsPublic, handler.StreamGetTweetsHandler(mw, tweetRepo))
	n.SetStreamHandler(stream.InfoPublic, handler.StreamGetInfoHandler(mw, n))

	nodeReadyChan <- authInfo
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
