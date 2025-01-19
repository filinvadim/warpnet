package handlers

import (
	"bytes"
	"fmt"
	api "github.com/filinvadim/warpnet/server/api-gen"
	"github.com/labstack/echo/v4"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
)

type StaticFolderOpener interface {
	Open(name string) (fs.File, error)
}

type StaticController struct {
	fileSystem fs.FS
	isFirstRun bool
}

func NewStaticController(isFirstRun bool, staticFolder StaticFolderOpener) *StaticController {
	pwd, _ := os.Getwd()
	log.Println("CURRENT DIRECTORY: ", pwd)
	fileSystem := echo.MustSubFS(staticFolder, "dist/")

	return &StaticController{fileSystem, isFirstRun}
}

func (c *StaticController) GetIndex(ctx echo.Context) error {
	f, err := c.fileSystem.Open("index.html")
	if err != nil {
		log.Println("failed opening index.html: ", err)
		return ctx.JSON(http.StatusInternalServerError, api.ErrorResponse{500, err.Error(), nil})
	}
	fi, _ := f.Stat()

	content, err := io.ReadAll(f)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.ErrorResponse{500, err.Error(), nil})
	}

	injectedContent := strings.Replace(
		string(content),
		"</head>",
		fmt.Sprintf("<script>window.isFirstRun = %t;</script></head>", c.isFirstRun),
		1,
	)

	tempFile := bytes.NewReader([]byte(injectedContent))

	ctx.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	ctx.Response().Header().Set("Pragma", "no-cache")
	ctx.Response().Header().Set("Expires", "0")

	http.ServeContent(ctx.Response(), ctx.Request(), fi.Name(), fi.ModTime(), tempFile)
	return nil
}

func (c *StaticController) GetStaticFile(ctx echo.Context, filePath string) error {
	f, err := c.fileSystem.Open(filePath)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.ErrorResponse{500, err.Error(), nil})
	}
	fi, _ := f.Stat()
	ff := f.(io.ReadSeeker)
	ctx.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	ctx.Response().Header().Set("Pragma", "no-cache")
	ctx.Response().Header().Set("Expires", "0")
	http.ServeContent(ctx.Response(), ctx.Request(), fi.Name(), fi.ModTime(), ff)
	return nil
}
