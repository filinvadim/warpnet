package handlers

import (
	"bytes"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/server/api-gen"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type StaticFolderOpener interface {
	Open(name string) (fs.File, error)
}

type StaticController struct {
	fileSystem               fs.FS
	isFirstRun               bool
	backendHost, backendPort string
}

func NewStaticController(
	isFirstRun bool,
	staticFolder StaticFolderOpener,
) *StaticController {
	pwd, _ := os.Getwd()
	log.Infoln("CURRENT DIRECTORY: ", pwd)
	fileSystem := echo.MustSubFS(staticFolder, "dist/")

	return &StaticController{
		fileSystem, isFirstRun,
		config.ConfigFile.Server.Host, config.ConfigFile.Server.Port,
	}
}

func (c *StaticController) GetIndex(ctx echo.Context) error {
	f, err := c.fileSystem.Open("index.html")
	if err != nil {
		log.Errorf("failed opening index.html: %v", err)
		return ctx.JSON(http.StatusInternalServerError, api.ErrorResponse{500, err.Error()})
	}
	fi, _ := f.Stat()

	content, err := io.ReadAll(f)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.ErrorResponse{500, err.Error()})
	}

	injectedContent := strings.Replace(
		string(content),
		"</head>",
		fmt.Sprintf(
			`<script>
			window.isFirstRun = %t;
			window.backendHost = %s;
			window.backendPort = %s;
		</script></head>`,
			c.isFirstRun,
			strconv.Quote(c.backendHost),
			strconv.Quote(c.backendPort),
		),
		1,
	)

	tempFile := bytes.NewReader([]byte(injectedContent))

	http.ServeContent(ctx.Response(), ctx.Request(), fi.Name(), fi.ModTime(), tempFile)
	return nil
}

func (c *StaticController) GetStaticFile(ctx echo.Context, filePath string) error {
	f, err := c.fileSystem.Open(filePath)
	if err != nil {
		f, err = c.fileSystem.Open("index.html")
		if err != nil {
			return ctx.JSON(http.StatusInternalServerError, api.ErrorResponse{500, err.Error()})
		}
	}
	fi, _ := f.Stat()
	ff := f.(io.ReadSeeker)

	http.ServeContent(ctx.Response(), ctx.Request(), fi.Name(), fi.ModTime(), ff)
	return nil
}
