package handlers

import (
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/dWighter/api/api"
	"github.com/filinvadim/dWighter/api/components"
	"github.com/filinvadim/dWighter/crypto"
	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/json"
	"github.com/labstack/echo/v4"
	"github.com/oapi-codegen/runtime/types"
	"io"
	"net/http"
	"time"
)

const apifyAddr = "https://api.ipify.org?format=json"

type apifyResponse struct {
	IP string `json:"ip"`
}

type AuthController struct {
	authRepo  *database.AuthRepo
	nodeRepo  *database.NodeRepo
	interrupt chan struct{}
}

func NewAuthController(
	authRepo *database.AuthRepo, nodeRepo *database.NodeRepo, interrupt chan struct{},
) *AuthController {
	return &AuthController{authRepo, nodeRepo, interrupt}
}

func (c *AuthController) PostV1ApiAuthLogin(ctx echo.Context) error {
	var req api.AuthRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	hashSum := crypto.ConvertToSHA256([]byte(req.Password + ":" + req.Username)) // aaaa + vadim
	if err := c.authRepo.InitWithPassword(hashSum); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	owner, err := c.authRepo.GetOwner()
	if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if owner != nil && owner.Username != req.Username {
		fmt.Println("OOOPS", owner)

		return ctx.JSON(http.StatusBadRequest, errors.New("wrong username"))
	}
	if owner != nil && owner.Username == req.Username {
		fmt.Println("HERE", owner)
		c.interrupt <- struct{}{}
		return ctx.JSON(http.StatusOK, owner)
	}
	u, err := c.authRepo.SetOwner(components.User{
		UserId:   nil,
		NodeId:   types.UUID{},
		Username: req.Username,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	_, err = c.nodeRepo.GetByUserId(*u.UserId)
	if err == nil {
		c.interrupt <- struct{}{}
		return ctx.JSON(http.StatusOK, u)
	}
	if !errors.Is(err, database.ErrNodeNotFound) {
		fmt.Println("failed to get node:", err)

		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	ip, err := getOwnIPAddress()
	if err != nil {
		fmt.Println("failed to get ip address")
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	fmt.Println("YOUR OWN IP", ip)

	now := time.Now()
	id, err := c.nodeRepo.Create(&components.Node{
		CreatedAt: &now,
		Ip:        ip,
		IsActive:  true,
		LastSeen:  now,
		Latency:   nil,
		OwnerId:   *u.UserId,
		Port:      "6969",
		Uptime:    func(i int64) *int64 { return &i }(0),
		IsOwned:   true,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	u.NodeId = id
	err = c.authRepo.UpdateOwner(u)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	// TODO broadcast id
	c.interrupt <- struct{}{}
	return ctx.JSON(http.StatusOK, u)
}

func getOwnIPAddress() (string, error) {
	resp, err := http.Get(apifyAddr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bt, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var ipResp apifyResponse
	err = json.JSON.Unmarshal(bt, &ipResp)
	return ipResp.IP, err
}

func (c *AuthController) PostV1ApiAuthLogout(ctx echo.Context) error {
	c.interrupt <- struct{}{}
	return ctx.NoContent(http.StatusOK)
}
