package example

import (
	"context"
	"github.com/filinvadim/warpnet/chat/token"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/filinvadim/warpnet/chat/chat"
	"github.com/filinvadim/warpnet/chat/controller"
	"github.com/filinvadim/warpnet/chat/database"
	"github.com/filinvadim/warpnet/chat/service"

	"go.uber.org/zap"
)

func main() {
	var (
		dbUser    = os.Getenv("DB_USER")
		dbPass    = os.Getenv("DB_PASS")
		dbName    = os.Getenv("DB_NAME")
		jwtSecret = os.Getenv("JWT_SECRET")
		jwtIssuer = os.Getenv("JWT_ISSUER")
		port      = ":4000"
	)

	db, err := database.New(dbUser, dbPass, dbName)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	signer := token.New(token.SignerOptions{
		Now: func() time.Time {
			return time.Now().UTC()
		},
		Issuer: jwtIssuer,
		TTL:    1 * time.Hour,
		Secret: []byte(jwtSecret),
	})

	c := chat.New(db, nil)
	defer c.Close()

	ctl := controller.New()

	getRoomsService := service.NewGetRoomsService(db)
	getConversationsService := service.NewGetConversationsService(db)
	postAuthorizeService := service.NewAuthorizeService(db)
	postLoginService := service.NewLoginService(db, signer)
	postRegisterService := service.NewRegisterService(db, signer)
	getUsersService := service.NewGetUsersService(db)
	handleFriendService := service.NewHandleFriendService(db)
	addFriendService := service.NewAddFriendService(db)
	getContactsService := service.NewGetContactsService(db)
	postRoomsService := service.NewPostRoomsService(db)

	getPostsService := service.NewGetPostsService(db)
	getPostService := service.NewGetPostService(db)
	createPostService := service.NewCreatePostService(db)
	updatePostService := service.NewUpdatePostService(db)
	deletePostService := service.NewDeletePostService(db)

	// Serve public files.
	router.ServeFiles("/public/*filepath", http.Dir("./public"))
	// router.GET("/", http.FileServer(http.Dir("./public")))

	router.GET("/ws", c.ServeWS(signer, db))
	router.POST("/auth", authorized(ctl.PostAuthorize(postAuthorizeService)))

	router.GET("/rooms", authorized(ctl.GetRooms(getRoomsService)))
	router.POST("/rooms", authorized(ctl.PostRooms(postRoomsService)))

	router.GET("/conversations/:id", authorized(ctl.GetConversations(getConversationsService)))
	router.POST("/register", ctl.PostRegister(postRegisterService))
	router.POST("/login", ctl.PostLogin(postLoginService))
	router.GET("/users", authorized(ctl.GetUsers(getUsersService)))

	router.POST("/friends/:id", authorized(ctl.PostFriendship(addFriendService)))
	router.PATCH("/friends/:id", authorized(ctl.PatchFriendship(handleFriendService)))

	router.GET("/contacts", authorized(ctl.GetContacts(getContactsService)))

	router.POST("/posts", authorized(ctl.CreatePost(createPostService)))
	router.GET("/posts", authorized(ctl.GetPosts(getPostsService)))
	router.GET("/posts/:id", authorized(ctl.GetPost(getPostService)))
	router.PATCH("/posts/:id", authorized(ctl.UpdatePost(updatePostService)))
	router.DELETE("/posts/:id", authorized(ctl.DeletePost(deletePostService)))

	srv := &http.Server{
		Addr:         port,
		Handler:      router,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit,
		syscall.SIGINT,
		syscall.SIGKILL,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		log.Printf("listening to port *%s. press ctrl + c to cancel.\n", port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("gracefully shut down application")
}
