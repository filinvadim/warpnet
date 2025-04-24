package member

import (
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/consensus"
	dht "github.com/filinvadim/warpnet/core/dhash-table"
	"github.com/filinvadim/warpnet/core/discovery"
	"github.com/filinvadim/warpnet/core/handler"
	"github.com/filinvadim/warpnet/core/mdns"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/node/base"
	"github.com/filinvadim/warpnet/core/pubsub"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/retrier"
	"github.com/filinvadim/warpnet/security"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type MemberNode struct {
	*base.WarpNode

	ctx           context.Context
	discService   DiscoveryHandler
	mdnsService   MDNSStarterCloser
	pubsubService PubSubProvider
	raft          ConsensusProvider
	dHashTable    DistributedHashTableCloser
	providerStore ProviderCacheCloser
	nodeRepo      ProviderCacheCloser
	retrier       retrier.Retrier
	userRepo      UserFetcher
}

func NewMemberNode(
	ctx context.Context,
	privKey warpnet.WarpPrivateKey,
	psk security.PSK,
	authRepo AuthProvider,
	db Storer,
) (_ *MemberNode, err error) {
	consensusRepo := database.NewConsensusRepo(db)
	nodeRepo := database.NewNodeRepo(db)
	store, err := warpnet.NewPeerstore(ctx, nodeRepo)
	if err != nil {
		return nil, err
	}

	userRepo := database.NewUserRepo(db)
	followRepo := database.NewFollowRepo(db)
	owner := authRepo.GetOwner()

	raft, err := consensus.NewRaft(
		ctx, consensusRepo, false,
		userRepo.ValidateUser,
	)
	if err != nil {
		return nil, fmt.Errorf("member: consensus initialization: %v", err)
	}

	discService := discovery.NewDiscoveryService(ctx, userRepo, nodeRepo)
	mdnsService := mdns.NewMulticastDNS(ctx, raft.AddVoter, discService.HandlePeerFound)
	pubsubService := pubsub.NewPubSub(ctx, followRepo, owner.UserId, raft.AddVoter, discService.HandlePeerFound)
	providerStore, err := dht.NewProviderCache(ctx, nodeRepo)
	if err != nil {
		return nil, fmt.Errorf("member: failed to init providers: %v", err)
	}

	dHashTable := dht.NewDHTable(
		ctx, nodeRepo, providerStore,
		raft.RemoveVoter, raft.AddVoter, discService.HandlePeerFound,
	)

	node, err := base.NewWarpNode(
		ctx,
		privKey,
		store,
		owner.UserId,
		psk,
		fmt.Sprintf("/ip4/%s/tcp/%s", config.ConfigFile.Node.Host, config.ConfigFile.Node.Port),
		dHashTable.StartRouting,
	)
	if err != nil {
		return nil, fmt.Errorf("member: failed to init node: %v", err)
	}

	println()
	fmt.Printf(
		"\033[1mNODE STARTED WITH ID %s AND ADDRESSES %v\033[0m\n",
		node.NodeInfo().ID, node.NodeInfo().Addrs,
	)
	println()

	mn := &MemberNode{
		WarpNode:      node,
		ctx:           ctx,
		discService:   discService,
		mdnsService:   mdnsService,
		pubsubService: pubsubService,
		raft:          raft,
		dHashTable:    dHashTable,
		providerStore: providerStore,
		nodeRepo:      nodeRepo,
		retrier:       retrier.New(time.Second, 10, retrier.ExponentialBackoff),
		userRepo:      userRepo,
	}

	mn.setupHandlers(authRepo, userRepo, followRepo, db)
	return mn, nil
}

func (m *MemberNode) setupHandlers(
	authRepo AuthProvider, userRepo UserProvider, followRepo FollowStorer, db Storer,
) {
	timelineRepo := database.NewTimelineRepo(db)
	tweetRepo := database.NewTweetRepo(db)
	replyRepo := database.NewRepliesRepo(db)
	likeRepo := database.NewLikeRepo(db)
	chatRepo := database.NewChatRepo(db)

	authNodeInfo := domain.AuthNodeInfo{
		Identity: domain.Identity{Owner: authRepo.GetOwner(), Token: authRepo.SessionToken()},
		NodeInfo: m.NodeInfo(),
	}

	mw := middleware.NewWarpMiddleware()
	logMw := mw.LoggingMiddleware
	authMw := mw.AuthMiddleware
	unwrapMw := mw.UnwrapStreamMiddleware
	m.SetStreamHandler(
		event.PUBLIC_POST_NODE_VERIFY,
		logMw(unwrapMw(handler.StreamVerifyHandler(m.raft))),
	)
	m.SetStreamHandler(
		event.PUBLIC_GET_INFO,
		logMw(handler.StreamGetInfoHandler(m, db, m.raft, m.discService.HandlePeerFound)),
	)
	m.SetStreamHandler(
		event.PRIVATE_POST_PAIR,
		logMw(authMw(unwrapMw(handler.StreamNodesPairingHandler(authNodeInfo)))),
	)
	m.SetStreamHandler(
		event.PRIVATE_GET_TIMELINE,
		logMw(authMw(unwrapMw(handler.StreamTimelineHandler(timelineRepo)))),
	)
	m.SetStreamHandler(
		event.PRIVATE_POST_TWEET,
		logMw(authMw(unwrapMw(handler.StreamNewTweetHandler(m.pubsubService, authRepo, tweetRepo, timelineRepo)))),
	)
	m.SetStreamHandler(
		event.PRIVATE_DELETE_TWEET,
		logMw(authMw(unwrapMw(handler.StreamDeleteTweetHandler(m.pubsubService, authRepo, tweetRepo, likeRepo)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_POST_REPLY,
		logMw(authMw(unwrapMw(handler.StreamNewReplyHandler(replyRepo, userRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_DELETE_REPLY,
		logMw(authMw(unwrapMw(handler.StreamDeleteReplyHandler(tweetRepo, userRepo, replyRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_POST_FOLLOW,
		logMw(authMw(unwrapMw(handler.StreamFollowHandler(m.pubsubService, followRepo, authRepo, userRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_POST_UNFOLLOW,
		logMw(authMw(unwrapMw(handler.StreamUnfollowHandler(m.pubsubService, followRepo, authRepo, userRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_GET_USER,
		logMw(authMw(unwrapMw(handler.StreamGetUserHandler(tweetRepo, followRepo, userRepo, authRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_GET_USERS,
		logMw(authMw(unwrapMw(handler.StreamGetUsersHandler(userRepo, authRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_GET_TWEETS,
		logMw(authMw(unwrapMw(handler.StreamGetTweetsHandler(tweetRepo, authRepo, userRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_GET_TWEET,
		logMw(authMw(unwrapMw(handler.StreamGetTweetHandler(tweetRepo)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_GET_TWEET_STATS,
		logMw(authMw(unwrapMw(handler.StreamGetTweetStatsHandler(likeRepo, tweetRepo, replyRepo)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_GET_REPLY,
		logMw(authMw(unwrapMw(handler.StreamGetReplyHandler(replyRepo)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_GET_REPLIES,
		logMw(authMw(unwrapMw(handler.StreamGetRepliesHandler(replyRepo)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_GET_FOLLOWERS,
		logMw(authMw(unwrapMw(handler.StreamGetFollowersHandler(authRepo, userRepo, followRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_GET_FOLLOWEES,
		logMw(authMw(unwrapMw(handler.StreamGetFolloweesHandler(authRepo, userRepo, followRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_POST_LIKE,
		logMw(authMw(unwrapMw(handler.StreamLikeHandler(likeRepo, userRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_POST_UNLIKE,
		logMw(authMw(unwrapMw(handler.StreamUnlikeHandler(likeRepo, userRepo, m)))),
	)
	m.SetStreamHandler(
		event.PRIVATE_POST_USER,
		logMw(authMw(unwrapMw(handler.StreamUpdateProfileHandler(authRepo, userRepo)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_POST_RETWEET,
		logMw(authMw(unwrapMw(handler.StreamNewReTweetHandler(authRepo, userRepo, tweetRepo, timelineRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_POST_UNRETWEET,
		logMw(authMw(unwrapMw(handler.StreamUnretweetHandler(authRepo, tweetRepo, userRepo, m)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_POST_CHAT,
		logMw(authMw(unwrapMw(handler.StreamCreateChatHandler(chatRepo, userRepo, authRepo, m)))),
	)
	m.SetStreamHandler(
		event.PRIVATE_DELETE_CHAT,
		logMw(authMw(unwrapMw(handler.StreamDeleteChatHandler(chatRepo)))),
	)
	m.SetStreamHandler(
		event.PRIVATE_GET_CHATS,
		logMw(authMw(unwrapMw(handler.StreamGetUserChatsHandler(chatRepo, authRepo)))),
	)
	m.SetStreamHandler(
		event.PUBLIC_POST_MESSAGE,
		logMw(authMw(unwrapMw(handler.StreamSendMessageHandler(chatRepo, userRepo, m)))),
	)
	m.SetStreamHandler(
		event.PRIVATE_DELETE_MESSAGE,
		logMw(authMw(unwrapMw(handler.StreamDeleteMessageHandler(chatRepo)))),
	)
	m.SetStreamHandler(
		event.PRIVATE_GET_MESSAGE,
		logMw(authMw(unwrapMw(handler.StreamGetMessageHandler(chatRepo, userRepo)))),
	)
	m.SetStreamHandler(
		event.PRIVATE_GET_MESSAGES,
		logMw(authMw(unwrapMw(handler.StreamGetMessagesHandler(chatRepo, userRepo)))),
	)
	m.SetStreamHandler(
		event.PRIVATE_GET_CHAT,
		logMw(authMw(unwrapMw(handler.StreamGetUserChatHandler(chatRepo, authRepo)))),
	)
}

func (m *MemberNode) Start(clientNode ClientNodeStreamer) error {
	go m.discService.Run(m)
	go m.mdnsService.Start(m)
	go m.pubsubService.Run(m, clientNode)

	if err := m.raft.Sync(m); err != nil {
		return fmt.Errorf("consensus: failed to sync: %v", err)
	}

	log.Debugln("SUPPORTED PROTOCOLS:", strings.Join(m.SupportedProtocols(), ","))

	ownerUser, err := m.userRepo.Get(m.NodeInfo().OwnerId)
	if err != nil {
		return fmt.Errorf("member: failed to get owner user: %v", err)
	}

	return m.raft.AskUserValidation(ownerUser)
}

func (m *MemberNode) Stop() {
	if m == nil {
		return
	}
	if m.discService != nil {
		m.discService.Close()
	}
	if m.mdnsService != nil {
		m.mdnsService.Close()
	}
	if m.pubsubService != nil {
		if err := m.pubsubService.Close(); err != nil {
			log.Errorf("member: failed to close pubsub: %v", err)
		}
	}
	if m.providerStore != nil {
		if err := m.providerStore.Close(); err != nil {
			log.Errorf("member: failed to close provider: %v", err)
		}
	}
	if m.dHashTable != nil {
		m.dHashTable.Close()
	}
	if m.raft != nil {
		m.raft.Shutdown()
	}
	if m.nodeRepo != nil {
		if err := m.nodeRepo.Close(); err != nil {
			log.Errorf("member: failed to close node repo: %v", err)
		}
	}

	m.StopNode()
}
