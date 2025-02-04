package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	embedded "github.com/filinvadim/warpnet"
	frontend "github.com/filinvadim/warpnet-frontend"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/consensus"
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
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/security"
	"github.com/filinvadim/warpnet/server/auth"
	"github.com/filinvadim/warpnet/server/handlers"
	"github.com/filinvadim/warpnet/server/server"
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
	security.EnableCoreDumps()
	isValid := security.VerifySelfSignature(embedded.GetSignature(), embedded.GetPublicKey())
	if !isValid {
		log.Infoln("invalid binary signature - TODO") // TODO
	}
	security.DisableCoreDumps()
	security.MustNotGDBAttached()
	selfHash, err := security.GetSelfHash(security.Member)
	if err != nil {
		log.Fatalf("fail to get self hash: %v", err)
	}
	log.Infoln("self hash:", selfHash) // TODO verify with network consensus

	conf, err := config.GetConfig()
	if err != nil {
		log.Fatalf("fail loading config: %v", err)
	}

	log.Infoln("config bootstrap nodes: ", conf.Node.Bootstrap)
	log.Infoln("Warpnet Version:", conf.Version)

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
	followRepo := database.NewFollowRepo(db)
	timelineRepo := database.NewTimelineRepo(db)
	tweetRepo := database.NewTweetRepo(db)
	replyRepo := database.NewRepliesRepo(db)
	consensusRepo := database.NewConsensusRepo(db)
	likeRepo := database.NewLikeRepo(db)

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
	staticCtrl := handlers.NewStaticController(conf, db.IsFirstRun(), frontend.GetStaticEmbedded())

	interfaceServer.RegisterHandlers(&API{
		staticCtrl,
		wsCtrl,
	})
	defer interfaceServer.Shutdown(ctx)
	go interfaceServer.Start()

	var authInfo domain.AuthNodeInfo
	select {
	case <-interruptChan:
		log.Infoln("logged out")
		return
	case authInfo = <-authReadyChan:
		log.Infoln("authentication was successful")
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
		selfHash,
		persLayer,
		conf,
		dHashTable.StartRouting,
	)
	if err != nil {
		log.Fatalf("failed to init node: %v", err)
	}
	defer n.Stop()

	go discService.Run(n)
	go mdnsService.Start(n)
	go pubsubService.Run(n)

	authInfo.Identity.Owner.NodeId = n.ID().String()
	authInfo.Identity.Owner.Ipv6 = n.IPv6()
	authInfo.Identity.Owner.Ipv4 = n.IPv4()
	authInfo.Version = conf.Version.String()

	mw := middleware.NewWarpMiddleware()
	logMw := mw.LoggingMiddleware
	authMw := mw.AuthMiddleware
	unwrapMw := mw.UnwrapStreamMiddleware

	n.SetStreamHandler(
		stream.PairPostPrivate,
		logMw(authMw(unwrapMw(handler.StreamNodesPairingHandler(authInfo)))),
	)
	n.SetStreamHandler(
		event.PRIVATE_GET_TIMELINE_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamTimelineHandler(timelineRepo)))),
	)
	n.SetStreamHandler(
		event.PRIVATE_POST_TWEET_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamNewTweetHandler(pubsubService, authRepo, tweetRepo, timelineRepo)))),
	)
	n.SetStreamHandler(
		event.PRIVATE_DELETE_TWEET_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamDeleteTweetHandler(pubsubService, authRepo, tweetRepo)))),
	)
	n.SetStreamHandler(
		event.PRIVATE_POST_REPLY_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamNewReplyHandler(pubsubService, authRepo, replyRepo)))),
	)
	n.SetStreamHandler(
		event.PRIVATE_DELETE_REPLY_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamDeleteReplyHandler(pubsubService, authRepo, replyRepo)))),
	)
	n.SetStreamHandler(
		event.PRIVATE_POST_FOLLOW_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamFollowHandler(pubsubService, followRepo)))),
	)
	n.SetStreamHandler(
		event.PRIVATE_POST_UNFOLLOW_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamUnfollowHandler(pubsubService, followRepo)))),
	)
	n.SetStreamHandler(
		event.PUBLIC_GET_USER_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamGetUserHandler(userRepo)))),
	)
	n.SetStreamHandler(
		event.PUBLIC_GET_USERS_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamGetUsersHandler(userRepo)))),
	)
	n.SetStreamHandler(
		event.PUBLIC_GET_TWEETS_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamGetTweetsHandler(tweetRepo)))),
	)
	n.SetStreamHandler(
		event.PUBLIC_GET_INFO_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamGetInfoHandler(n)))),
	)
	n.SetStreamHandler(
		event.PUBLIC_GET_TWEET_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamGetTweetHandler(tweetRepo)))),
	)
	n.SetStreamHandler(
		event.PUBLIC_GET_REPLY_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamGetReplyHandler(replyRepo)))),
	)
	n.SetStreamHandler(
		event.PUBLIC_GET_REPLIES_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamGetRepliesHandler(replyRepo)))),
	)
	n.SetStreamHandler(
		event.PUBLIC_GET_FOLLOWERS_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamGetFollowersHandler(authRepo, userRepo, followRepo, n)))),
	)
	n.SetStreamHandler(
		event.PUBLIC_GET_FOLLOWEES_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamGetFolloweesHandler(authRepo, userRepo, followRepo, n)))),
	)

	n.SetStreamHandler(
		event.PRIVATE_POST_LIKE_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamLikeHandler(likeRepo, pubsubService)))),
	)
	n.SetStreamHandler(
		event.PRIVATE_POST_UNLIKE_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamUnlikeHandler(likeRepo, pubsubService)))),
	)
	n.SetStreamHandler(
		event.PUBLIC_GET_LIKESNUM_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamGetLikesNumHandler(likeRepo)))),
	)
	n.SetStreamHandler(
		event.PUBLIC_GET_LIKERS_1_0_0,
		logMw(authMw(unwrapMw(handler.StreamGetLikersHandler(likeRepo, userRepo)))),
	)
	log.Infoln("SUPPORTED PROTOCOLS:", n.SupportedProtocols())

	nodeReadyChan <- authInfo

	bootstrapAddrs, _ := conf.Node.AddrInfos()
	raft, err := consensus.NewRaft(ctx, n, consensusRepo, false)
	if err != nil {
		log.Fatal(err)
	}
	raft.Negotiate(bootstrapAddrs)
	defer raft.Shutdown()
	<-interruptChan
	log.Infoln("interrupted...")
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
