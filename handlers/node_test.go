package handlers_test

import (
	"bytes"
	"encoding/json"
	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/handlers"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// setupTest initializes a new Echo instance and test database
func setupTest(t *testing.T) (*echo.Echo, *database.NodeRepo, func()) {
	e := echo.New()

	path := "../var/handlertest"
	db := storage.New("nodetest", path, false, true, "error")
	nodeRepo := database.NewNodeRepo(db)

	cleanup := func() {
		db.Close()
		os.RemoveAll(path)
	}
	return e, nodeRepo, cleanup
}

// TestPostNodes creates a new node and checks if it's added correctly
func TestPostNodes(t *testing.T) {
	e, nodeRepo, cleanup := setupTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewNodeController(nodeRepo)

	// Пример ноды
	node := server.Node{
		Ip:      "127.0.0.1",
		OwnerId: "TestPostNodes",
	}

	// Создаем HTTP запрос
	nodeJson, _ := json.Marshal(node)
	req := httptest.NewRequest(http.MethodPost, "/nodes", bytes.NewReader(nodeJson))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	// Создаем Echo контекст
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.PostNodes(ctx)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var createdNode server.Node
		if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createdNode)) {
			assert.Equal(t, node.Ip, createdNode.Ip)
			assert.Equal(t, node.OwnerId, createdNode.OwnerId)
		}
	}
}

// TestGetNodes checks if the nodes are retrieved correctly
func TestGetNodes(t *testing.T) {
	e, nodeRepo, cleanup := setupTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewNodeController(nodeRepo)

	// Добавляем тестовую ноду в базу данных
	node := server.Node{
		Ip:      "127.0.0.1",
		OwnerId: "TestGetNodes",
	}
	_ = nodeRepo.Create(node)

	// Создаем HTTP запрос
	req := httptest.NewRequest(http.MethodGet, "/nodes", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.GetNodes(ctx)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var nodes []server.Node
		if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &nodes)) {
			assert.Len(t, nodes, 1)
			assert.Equal(t, node.Ip, nodes[0].Ip)
		}
	}
}

// TestPostNodesPing tests the ping functionality of a node
func TestPostNodesPing(t *testing.T) {
	e, nodeRepo, cleanup := setupTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewNodeController(nodeRepo)

	// Добавляем тестовую ноду в базу данных
	node := server.Node{
		Ip:      "127.0.0.1",
		OwnerId: "Test Node",
	}
	_ = nodeRepo.Create(node)

	// Создаем HTTP запрос для пинга
	pingRequest := server.Node{
		Ip: "127.0.0.1",
	}
	pingJson, _ := json.Marshal(pingRequest)
	req := httptest.NewRequest(http.MethodPost, "/nodes/ping", bytes.NewReader(pingJson))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Выполняем запрос пинга
	if assert.NoError(t, controller.PostNodesPing(ctx)) {
		assert.Equal(t, http.StatusOK, rec.Code)

		// Проверяем, что время последнего пинга было обновлено
		updatedNode, err := nodeRepo.GetByIP("127.0.0.1")
		assert.NoError(t, err)
		assert.WithinDuration(t, time.Now(), updatedNode.LastSeen, time.Second)
	}
}
