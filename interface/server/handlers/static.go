package handlers

import (
	"fmt"
	"github.com/filinvadim/warpnet/config"
	domainGen "github.com/filinvadim/warpnet/domain-gen"
	"github.com/labstack/echo/v4"
	"io"
	"io/fs"
	"net/http"
)

type StaticFolderOpener interface {
	Open(name string) (fs.File, error)
}

type StaticController struct {
	fileSystem fs.FS
}

func NewStaticController(staticFolder StaticFolderOpener) *StaticController {
	fileSystem := echo.MustSubFS(staticFolder, config.StaticDirPath)
	return &StaticController{fileSystem}
}

func (c *StaticController) GetIndex(ctx echo.Context) error {
	ctx.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	ctx.Response().Header().Set("Pragma", "no-cache")
	ctx.Response().Header().Set("Expires", "0")
	f, err := c.fileSystem.Open(config.StaticDirPath + "index.html")
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domainGen.Error{500, err.Error()})
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
	fullPath := fmt.Sprintf("%s%s%s", config.StaticDirPath, "static/", filePath)
	f, err := c.fileSystem.Open(fullPath)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, domainGen.Error{500, err.Error()})
	}
	fi, _ := f.Stat()
	ff := f.(io.ReadSeeker)
	http.ServeContent(ctx.Response(), ctx.Request(), fi.Name(), fi.ModTime(), ff)
	return nil
}
