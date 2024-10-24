package handlers

import "github.com/labstack/echo/v4"

type StaticController struct {
}

func NewStaticController() *StaticController {
	return &StaticController{}
}

func (c *StaticController) GetIndex(ctx echo.Context) error {
	return ctx.File("static/index.html")
}
