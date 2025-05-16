/*

 Warpnet - Decentralized Social Network
 Copyright (C) 2025 Vadim Filin, https://github.com/Warp-net,
 <github.com.mecdy@passmail.net>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU General Public License for more details.

 You should have received a copy of the GNU General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

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
	"github.com/filinvadim/warpnet/metrics"
	"github.com/filinvadim/warpnet/security"
	"github.com/filinvadim/warpnet/server/auth"
	"github.com/filinvadim/warpnet/server/handlers"
	"github.com/filinvadim/warpnet/server/server"
	writer "github.com/ipfs/go-log/writer"
	log "github.com/sirupsen/logrus"
	"time"

	//"net/http"
	//_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

//func init() {
//	go func() {
//		http.ListenAndServe("localhost:8080", nil)
//	}()
//}

type API struct {
	*handlers.StaticController
	*handlers.WSController
}

func main() {
	defer closeWriter()
	appPath := getAppPath()

	psk, err := security.GeneratePSK(root.GetCodeBase(), config.Config().Version)
	if err != nil {
		log.Fatal(err)
	}

	lvl, err := log.ParseLevel(config.Config().Logging.Level)
	if err != nil {
		lvl = log.InfoLevel
	}
	log.SetLevel(lvl)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.DateTime,
	})
	if !config.Config().Node.IsTestnet() {
		logDir := filepath.Join(appPath, "log")
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			log.Fatal(err)
		}
		logPath := filepath.Join(logDir, fmt.Sprintf("%s.log", time.Now().Format(time.DateOnly)))
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	var interruptChan = make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := storage.New(appPath, false, config.Config().Database.DirName)
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	authRepo := database.NewAuthRepo(db)
	userRepo := database.NewUserRepo(db)

	var readyChan = make(chan domain.AuthNodeInfo, 1)
	defer close(readyChan)

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
	defer clientNode.Stop()

	authService := auth.NewAuthService(authRepo, userRepo, interruptChan, readyChan)
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
	case serverNodeAuthInfo = <-readyChan:
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

	readyChan <- serverNodeAuthInfo

	if err := clientNode.Pair(serverNodeAuthInfo); err != nil {
		log.Fatalf("failed to init client node: %v", err)
	}

	m := metrics.NewMetricsClient(
		config.Config().Node.Metrics.Server, serverNodeAuthInfo.Identity.Owner.NodeId, false,
	)
	m.PushStatusOnline()
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

// TODO temp. Check for https://github.com/libp2p/go-libp2p-kad-dht/issues/1073
func closeWriter() {
	defer func() { recover() }()
	_ = writer.WriterGroup.Close()
}
