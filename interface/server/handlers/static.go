package handlers

import (
	"fmt"
	"github.com/filinvadim/dWighter/config"
	"github.com/labstack/echo/v4"
)

type StaticController struct {
}

func NewStaticController() *StaticController {
	return &StaticController{}
}

func (c *StaticController) GetIndex(ctx echo.Context) error {
	ctx.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	ctx.Response().Header().Set("Pragma", "no-cache")
	ctx.Response().Header().Set("Expires", "0")
	return ctx.File(config.StaticDirPath + "index.html")
}

func (c *StaticController) GetStaticFile(ctx echo.Context, filePath string) error {
	ctx.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
	ctx.Response().Header().Set("Pragma", "no-cache")
	ctx.Response().Header().Set("Expires", "0")
	fullPath := fmt.Sprintf("%s%s%s", config.StaticDirPath, "static/", filePath)
	return ctx.File(fullPath)
}
