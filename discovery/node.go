package discovery

import (
	"github.com/filinvadim/dWighter/database"
	"github.com/labstack/echo/v4"
	"net/http"
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
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}
	uniqueNodes := make(map[string]api.Node, len(nodes))
	for _, n := range nodes {
		uniqueNodes[n.Ip] = n
	}

	// Преобразуем map обратно в срез
	result := make([]api.Node, 0, len(uniqueNodes))
	for _, node := range uniqueNodes {
		result = append(result, node)
	}

	return ctx.JSON(http.StatusOK, result)
}

// PostNodes creates a new node
func (c *NodeController) PostNodes(ctx echo.Context) error {
	var node api.Node
	err := ctx.Bind(&node)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	// Add node to the database
	_, err = c.nodeRepo.Create(node)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, api.Error{Code: http.StatusInternalServerError, Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, node)
}

func (c *NodeController) GetNodesPing(ctx echo.Context) error {
	return ctx.NoContent(http.StatusOK)
}
