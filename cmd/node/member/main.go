package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	root "github.com/filinvadim/warpnet"
	frontend "github.com/filinvadim/warpnet-frontend"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/consensus"
	dht "github.com/filinvadim/warpnet/core/dhash-table"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/handler"
	"github.com/filinvadim/warpnet/core/mdns"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/node/client"
	"github.com/filinvadim/warpnet/core/node/member"
	"github.com/filinvadim/warpnet/core/pubsub"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/security"
	"github.com/filinvadim/warpnet/server/auth"
	"github.com/filinvadim/warpnet/server/handlers"
	"github.com/filinvadim/warpnet/server/server"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type API struct {
	*handlers.StaticController
	*handlers.WSController
}

func main() {
	codeHash, err := security.GetCodebaseHash(root.GetCodeBase())
	if err != nil {
		panic(err)
	}

	log.Infof("codebase hash: %x", codeHash)
	log.Infoln("config bootstrap nodes: ", config.ConfigFile.Node.Bootstrap)
	log.Infoln("Warpnet version:", config.ConfigFile.Version)

	lvl, err := log.ParseLevel(config.ConfigFile.Logging.Level)
	if err != nil {
		lvl = log.InfoLevel
	}
	log.SetLevel(lvl)

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, dbCloser, err := storage.New(getAppPath(), false, config.ConfigFile.Database.DirName)
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
	chatRepo := database.NewChatRepo(db)

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

	clientNode, err := client.NewClientNode(ctx)
	if err != nil {
		log.Fatalf("failed to init client node: %v", err)
	}

	userPersistency := struct {
		*database.AuthRepo
		*database.UserRepo
	}{
		authRepo, userRepo,
	}

	authService := auth.NewAuthService(userPersistency, interruptChan, nodeReadyChan, authReadyChan)
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
		ctx, persLayer, providerStore, codeHash,
		discService.HandlePeerFound, nil,
	)
	defer dHashTable.Close()

	serverNode, err := member.NewMemberNode(
		ctx,
		privKey,
		string(codeHash),
		persLayer,
		dHashTable.StartRouting,
	)
	if err != nil {
		log.Fatalf("failed to init node: %v", err)
	}
	defer serverNode.Stop()

	serverNodeAuthInfo.Identity.Owner.NodeId = serverNode.ID().String()
	serverNodeAuthInfo.Identity.Owner.Ipv6 = serverNode.IPv6()
	serverNodeAuthInfo.Identity.Owner.Ipv4 = serverNode.IPv4()
	serverNodeAuthInfo.Version = config.ConfigFile.Version.String()

	mw := middleware.NewWarpMiddleware()
	logMw := mw.LoggingMiddleware
	authMw := mw.AuthMiddleware
	unwrapMw := mw.UnwrapStreamMiddleware

	serverNode.SetStreamHandler(
		event.PUBLIC_GET_INFO,
		logMw(handler.StreamGetInfoHandler(serverNode, discService.HandlePeerFound)),
	)
	serverNode.SetStreamHandler(
		event.PRIVATE_POST_PAIR,
		logMw(authMw(unwrapMw(handler.StreamNodesPairingHandler(serverNodeAuthInfo)))),
	)
	serverNode.SetStreamHandler(
		event.PRIVATE_GET_TIMELINE,
		logMw(authMw(unwrapMw(handler.StreamTimelineHandler(timelineRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PRIVATE_POST_TWEET,
		logMw(authMw(unwrapMw(handler.StreamNewTweetHandler(pubsubService, authRepo, tweetRepo, timelineRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PRIVATE_DELETE_TWEET,
		logMw(authMw(unwrapMw(handler.StreamDeleteTweetHandler(pubsubService, authRepo, tweetRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_POST_REPLY,
		logMw(authMw(unwrapMw(handler.StreamNewReplyHandler(replyRepo, userRepo, tweetRepo, serverNode)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_DELETE_REPLY,
		logMw(authMw(unwrapMw(handler.StreamDeleteReplyHandler(tweetRepo, userRepo, replyRepo, serverNode)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_POST_FOLLOW,
		logMw(authMw(unwrapMw(handler.StreamFollowHandler(pubsubService, serverNode, userRepo, followRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_POST_UNFOLLOW,
		logMw(authMw(unwrapMw(handler.StreamUnfollowHandler(pubsubService, serverNode, userRepo, followRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_GET_USER,
		logMw(authMw(unwrapMw(handler.StreamGetUserHandler(tweetRepo, followRepo, userRepo, authRepo, serverNode)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_GET_USERS,
		logMw(authMw(unwrapMw(handler.StreamGetUsersHandler(userRepo, authRepo, serverNode)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_GET_TWEETS,
		logMw(authMw(unwrapMw(handler.StreamGetTweetsHandler(tweetRepo, authRepo, userRepo, serverNode)))),
	)

	serverNode.SetStreamHandler(
		event.PUBLIC_GET_TWEET,
		logMw(authMw(unwrapMw(handler.StreamGetTweetHandler(tweetRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_GET_REPLY,
		logMw(authMw(unwrapMw(handler.StreamGetReplyHandler(replyRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_GET_REPLIES,
		logMw(authMw(unwrapMw(handler.StreamGetRepliesHandler(replyRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_GET_FOLLOWERS,
		logMw(authMw(unwrapMw(handler.StreamGetFollowersHandler(authRepo, userRepo, followRepo, serverNode)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_GET_FOLLOWEES,
		logMw(authMw(unwrapMw(handler.StreamGetFolloweesHandler(authRepo, userRepo, followRepo, serverNode)))),
	)

	serverNode.SetStreamHandler(
		event.PUBLIC_POST_LIKE,
		logMw(authMw(unwrapMw(handler.StreamLikeHandler(likeRepo, userRepo, serverNode)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_POST_UNLIKE,
		logMw(authMw(unwrapMw(handler.StreamUnlikeHandler(likeRepo, userRepo, serverNode)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_GET_LIKESCOUNT,
		logMw(authMw(unwrapMw(handler.StreamGetLikesNumHandler(likeRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_GET_LIKERS,
		logMw(authMw(unwrapMw(handler.StreamGetLikersHandler(likeRepo, userRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PRIVATE_POST_USER,
		logMw(authMw(unwrapMw(handler.StreamUpdateProfileHandler(authRepo, userRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_GET_RETWEETSCOUNT,
		logMw(authMw(unwrapMw(handler.StreamGetReTweetsCountHandler(tweetRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_POST_RETWEET,
		logMw(authMw(unwrapMw(handler.StreamNewReTweetHandler(authRepo, userRepo, tweetRepo, timelineRepo, serverNode)))),
	)

	serverNode.SetStreamHandler(
		event.PUBLIC_POST_UNRETWEET,
		logMw(authMw(unwrapMw(handler.StreamUnretweetHandler(authRepo, tweetRepo, userRepo, serverNode)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_GET_RETWEETERS,
		logMw(authMw(unwrapMw(handler.StreamGetRetweetersHandler(tweetRepo, userRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_POST_CHAT,
		logMw(authMw(unwrapMw(handler.StreamCreateChatHandler(chatRepo, userRepo, serverNode)))),
	)
	serverNode.SetStreamHandler(
		event.PRIVATE_DELETE_CHAT,
		logMw(authMw(unwrapMw(handler.StreamDeleteChatHandler(chatRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PRIVATE_GET_CHATS,
		logMw(authMw(unwrapMw(handler.StreamGetUserChatsHandler(chatRepo, userRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PUBLIC_POST_MESSAGE,
		logMw(authMw(unwrapMw(handler.StreamSendMessageHandler(chatRepo, userRepo, serverNode)))),
	)
	serverNode.SetStreamHandler(
		event.PRIVATE_DELETE_MESSAGE,
		logMw(authMw(unwrapMw(handler.StreamDeleteMessageHandler(chatRepo, userRepo, serverNode)))),
	)
	serverNode.SetStreamHandler(
		event.PRIVATE_GET_MESSAGE,
		logMw(authMw(unwrapMw(handler.StreamGetMessageHandler(chatRepo, userRepo)))),
	)
	serverNode.SetStreamHandler(
		event.PRIVATE_GET_MESSAGES,
		logMw(authMw(unwrapMw(handler.StreamGetMessagesHandler(chatRepo, userRepo)))),
	)

	serverNode.SetStreamHandler(
		event.PRIVATE_GET_CHAT,
		logMw(authMw(unwrapMw(handler.StreamGetUserChatHandler(chatRepo, authRepo)))),
	)
	log.Infoln("SUPPORTED PROTOCOLS:", strings.Join(serverNode.SupportedProtocols(), ","))

	nodeReadyChan <- serverNodeAuthInfo

	if err := clientNode.Pair(serverNodeAuthInfo); err != nil {
		log.Fatalf("failed to init client node: %v", err)
	}
	defer clientNode.Stop()

	go discService.Run(serverNode)
	go mdnsService.Start(serverNode)
	go pubsubService.Run(serverNode, clientNode, authRepo, followRepo)

	bootstrapAddrs, _ := config.ConfigFile.Node.AddrInfos()
	raft, err := consensus.NewRaft(ctx, serverNode, consensusRepo, false)
	if err != nil {
		log.Fatal(err)
	}
	raft.Negotiate(bootstrapAddrs)
	defer raft.Shutdown()

	newState, err := raft.Commit(consensus.ConsensusDefaultState{"test": time.Now().String()})
	fmt.Println(newState, err, "???????????????????????")
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
