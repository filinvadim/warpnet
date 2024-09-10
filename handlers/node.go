package handlers

import (
	"github.com/filinvadim/dWighter/api/server"
	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
	"net/http"
	"time"
)

type NodeController struct {
	nodeRepo *database.NodeRepo
}

func NewNodeController(nodeRepo *database.NodeRepo) *NodeController {
	return &NodeController{nodeRepo}
}

// GetNodes retrieves the list of all nodes
func (c *NodeController) GetNodes(ctx echo.Context) error {
	nodes, err := c.nodeRepo.List()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, server.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	uniqueNodes := make(map[string]server.Node, len(nodes))
	for _, n := range nodes {
		uniqueNodes[n.Ip] = n
	}

	// Преобразуем map обратно в срез
	result := make([]server.Node, 0, len(uniqueNodes))
	for _, node := range uniqueNodes {
		result = append(result, node)
	}

	return ctx.JSON(http.StatusOK, result)
}

// PostNodes creates a new node
func (c *NodeController) PostNodes(ctx echo.Context) error {
	var node server.Node
	err := ctx.Bind(&node)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, server.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	// Add node to the database
	err = c.nodeRepo.Create(node)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, server.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, node)
}

// PostNodesPing handles a ping request to a node
func (c *NodeController) PostNodesPing(ctx echo.Context) error {
	var pingRequest server.Node
	err := ctx.Bind(&pingRequest)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, server.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	// Handle ping logic (for example, updating the node's last ping time)
	node, err := c.nodeRepo.GetByIP(pingRequest.Ip)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, server.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	node.LastSeen = time.Now()
	err = c.nodeRepo.Update(node)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, server.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.NoContent(http.StatusOK)
}

func (c *NodeController) GetNodesPing(ctx echo.Context, _ server.GetNodesPingParams) error {
	return ctx.NoContent(http.StatusOK)
}
