package handlers_test

import (
	"bytes"
	"encoding/json"
	domain_gen "github.com/filinvadim/warpnet/domain-gen"
	"github.com/filinvadim/warpnet/interface/api-gen"
	"github.com/filinvadim/warpnet/interface/server/handlers"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// setupUserTest initializes a new Echo instance and test database for users and follows
func setupUserTest(t *testing.T) (*echo.Echo, *database.UserRepo, *database.FollowRepo, *database.NodeRepo, func()) {
	e := echo.New()

	// Инициализация тестовой базы данных для пользователей и подписок
	path := "../var/handlertest"
	db := storage.New(path, true)
	db.Run("", "")

	userRepo := database.NewUserRepo(db)
	followRepo := database.NewFollowRepo(db)
	nodeRepo := database.NewNodeRepo(db)

	// Возвращаем echo, репозитории и функцию очистки базы данных
	cleanup := func() {
		db.Close()
		os.RemoveAll(path)
		t.Log("CLEANED")
	}
	return e, userRepo, followRepo, nodeRepo, cleanup
}

// TestPostUser tests the creation of a new user
func TestPostUser(t *testing.T) {
	e, _, _, _, cleanup := setupUserTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewUserController(nil)

	userId := uuid.New().String()
	user := domain_gen.User{
		Id:       userId,
		Username: "testuser",
	}

	// Создаем HTTP запрос
	userJson, _ := json.Marshal(user)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(userJson))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	// Создаем контекст Echo
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.PostV1ApiUsers(ctx, api.PostV1ApiUsersParams{})) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var createdUser domain_gen.User
		if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createdUser)) {
			assert.Equal(t, user.Username, createdUser.Username)
			assert.Equal(t, user.Id, createdUser.Id)
		}
	}
}

// TestGetUser tests retrieving a user by userId
func TestGetUser(t *testing.T) {
	e, userRepo, _, _, cleanup := setupUserTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewUserController(nil)

	userId := uuid.New().String()
	user := domain_gen.User{
		Id:       userId,
		Username: "testuser",
	}

	// Добавляем пользователя в базу данных
	u, _ := userRepo.Create(user)
	user.Id = u.Id

	// Создаем HTTP запрос для получения пользователя по userId
	req := httptest.NewRequest(http.MethodGet, "/users/"+user.Id, nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.GetV1ApiUsersUserId(ctx, user.Id, api.GetV1ApiUsersUserIdParams{})) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var fetchedUser domain_gen.User
		if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &fetchedUser)) {
			assert.Equal(t, user.Username, fetchedUser.Username)
			assert.Equal(t, user.Id, fetchedUser.Id)
		}
	}
}

// TestFollowUser tests following another user
func TestFollowUser(t *testing.T) {
	e, userRepo, _, _, cleanup := setupUserTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewUserController(nil)

	userId := uuid.New().String()
	reader := domain_gen.User{
		Id:       userId,
		Username: "reader",
	}
	userId = uuid.New().String()
	writer := domain_gen.User{
		Id:       userId,
		Username: "writer",
	}

	// Добавляем пользователей в базу данных
	r, _ := userRepo.Create(reader)
	w, _ := userRepo.Create(writer)
	reader.Id = r.Id
	writer.Id = w.Id
	// Пример запроса на подписку
	followRequest := domain_gen.FollowRequest{
		ReaderId: reader.Id,
		WriterId: writer.Id,
	}

	// Создаем HTTP запрос на подписку
	followJson, _ := json.Marshal(followRequest)
	req := httptest.NewRequest(http.MethodPost, "/users/follow", bytes.NewReader(followJson))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.PostV1ApiUsersFollow(ctx, api.PostV1ApiUsersFollowParams{})) {
		assert.Equal(t, http.StatusCreated, rec.Code)

		// Проверяем, что подписка существует
		//following, err := followRepo.GetWriters(*reader.UserId)
		//assert.NoError(t, err)
		//assert.Contains(t, following, writer.UserId)
	}
}

// TestUnfollowUser tests unfollowing a user
func TestUnfollowUser(t *testing.T) {
	e, userRepo, _, _, cleanup := setupUserTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewUserController(nil)

	userId := uuid.New().String()
	reader := domain_gen.User{
		Id:       userId,
		Username: "reader",
	}
	userId = uuid.New().String()
	writer := domain_gen.User{
		Id:       userId,
		Username: "writer",
	}

	// Добавляем пользователей в базу данных
	r, _ := userRepo.Create(reader)
	w, _ := userRepo.Create(writer)
	reader.Id = r.Id
	writer.Id = w.Id

	// Подписываем reader на writer
	//_ = followRepo.Follow(*reader.UserId, *writer.UserId)

	// Пример запроса на отписку
	unfollowRequest := domain_gen.FollowRequest{
		ReaderId: reader.Id,
		WriterId: writer.Id,
	}

	// Создаем HTTP запрос на отписку
	unfollowJson, _ := json.Marshal(unfollowRequest)
	req := httptest.NewRequest(http.MethodPost, "/users/unfollow", bytes.NewReader(unfollowJson))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.PostV1ApiUsersUnfollow(ctx, api.PostV1ApiUsersUnfollowParams{})) {
		assert.Equal(t, http.StatusOK, rec.Code)

		// Проверяем, что подписка была удалена
		//following, err := followRepo.GetWriters(*reader.UserId)
		//assert.NoError(t, err)
		//assert.NotContains(t, following, writer.UserId)
	}
}
