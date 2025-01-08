package handlers

import (
	api "github.com/filinvadim/warpnet/server/api-gen"
	"github.com/labstack/echo/v4"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
)

type StaticFolderOpener interface {
	Open(name string) (fs.File, error)
}

type StaticController struct {
	fileSystem fs.FS
}

func NewStaticController(staticFolder StaticFolderOpener) *StaticController {
	pwd, _ := os.Getwd()
	log.Println("CURRENT DIRECTORY: ", pwd)
	fileSystem := echo.MustSubFS(staticFolder, "server/frontend/dist/")
	return &StaticController{fileSystem}
}

func (c *StaticController) GetIndex(ctx echo.Context) error {
	ctx.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	ctx.Response().Header().Set("Pragma", "no-cache")
	ctx.Response().Header().Set("Expires", "0")

	f, err := c.fileSystem.Open("index.html")
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{500, err.Error()})
	}
	fi, _ := f.Stat()
	ff := f.(io.ReadSeeker)
	http.ServeContent(ctx.Response(), ctx.Request(), fi.Name(), fi.ModTime(), ff)
	return nil
}

func (c *StaticController) GetStaticFile(ctx echo.Context, filePath string) error {
	ctx.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	ctx.Response().Header().Set("Pragma", "no-cache")
	ctx.Response().Header().Set("Expires", "0")
	f, err := c.fileSystem.Open(filePath)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{500, err.Error()})
	}
	fi, _ := f.Stat()
	ff := f.(io.ReadSeeker)
	http.ServeContent(ctx.Response(), ctx.Request(), fi.Name(), fi.ModTime(), ff)
	return nil
}
