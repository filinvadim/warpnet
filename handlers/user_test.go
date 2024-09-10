package handlers_test

import (
	"bytes"
	"encoding/json"
	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/handlers"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// setupUserTest initializes a new Echo instance and test database for users and follows
func setupUserTest(t *testing.T) (*echo.Echo, *database.UserRepo, *database.FollowRepo, func()) {
	e := echo.New()

	// Инициализация тестовой базы данных для пользователей и подписок
	path := "../var/handlertest"
	db := storage.New("tweettest", path, false, true, "error")
	userRepo := database.NewUserRepo(db)
	followRepo := database.NewFollowRepo(db)

	// Возвращаем echo, репозитории и функцию очистки базы данных
	cleanup := func() {
		db.Close()
		os.RemoveAll(path)
		t.Log("CLEANED")
	}
	return e, userRepo, followRepo, cleanup
}

// TestPostUser tests the creation of a new user
func TestPostUser(t *testing.T) {
	e, userRepo, followRepo, cleanup := setupUserTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewUserController(userRepo, followRepo)

	// Пример пользователя
	user := server.User{
		UserId:   uuid.New().String(),
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
	if assert.NoError(t, controller.PostUsers(ctx)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var createdUser server.User
		if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createdUser)) {
			assert.Equal(t, user.Username, createdUser.Username)
			assert.Equal(t, user.UserId, createdUser.UserId)
		}
	}
}

// TestGetUser tests retrieving a user by userId
func TestGetUser(t *testing.T) {
	e, userRepo, followRepo, cleanup := setupUserTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewUserController(userRepo, followRepo)

	// Пример пользователя
	user := server.User{
		UserId:   uuid.New().String(),
		Username: "testuser",
	}

	// Добавляем пользователя в базу данных
	u, _ := userRepo.Create(user)
	user.UserId = u.UserId

	// Создаем HTTP запрос для получения пользователя по userId
	req := httptest.NewRequest(http.MethodGet, "/users/"+user.UserId, nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.GetUsersUserId(ctx, user.UserId)) {
		assert.Equal(t, http.StatusOK, rec.Code)
		var fetchedUser server.User
		if assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &fetchedUser)) {
			assert.Equal(t, user.Username, fetchedUser.Username)
			assert.Equal(t, user.UserId, fetchedUser.UserId)
		}
	}
}

// TestFollowUser tests following another user
func TestFollowUser(t *testing.T) {
	e, userRepo, followRepo, cleanup := setupUserTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewUserController(userRepo, followRepo)

	// Пример пользователей
	reader := server.User{
		UserId:   uuid.New().String(),
		Username: "reader",
	}
	writer := server.User{
		UserId:   uuid.New().String(),
		Username: "writer",
	}

	// Добавляем пользователей в базу данных
	r, _ := userRepo.Create(reader)
	w, _ := userRepo.Create(writer)
	reader.UserId = r.UserId
	writer.UserId = w.UserId
	// Пример запроса на подписку
	followRequest := server.FollowRequest{
		ReaderId: reader.UserId,
		WriterId: writer.UserId,
	}

	// Создаем HTTP запрос на подписку
	followJson, _ := json.Marshal(followRequest)
	req := httptest.NewRequest(http.MethodPost, "/users/follow", bytes.NewReader(followJson))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.PostUsersFollow(ctx)) {
		assert.Equal(t, http.StatusCreated, rec.Code)

		// Проверяем, что подписка существует
		following, err := followRepo.GetWriters(reader.UserId)
		assert.NoError(t, err)
		assert.Contains(t, following, writer.UserId)
	}
}

// TestUnfollowUser tests unfollowing a user
func TestUnfollowUser(t *testing.T) {
	e, userRepo, followRepo, cleanup := setupUserTest(t)
	defer cleanup()

	// Создаем контроллер
	controller := handlers.NewUserController(userRepo, followRepo)

	// Пример пользователей
	reader := server.User{
		UserId:   uuid.New().String(),
		Username: "reader",
	}
	writer := server.User{
		UserId:   uuid.New().String(),
		Username: "writer",
	}

	// Добавляем пользователей в базу данных
	r, _ := userRepo.Create(reader)
	w, _ := userRepo.Create(writer)
	reader.UserId = r.UserId
	writer.UserId = w.UserId

	// Подписываем reader на writer
	_ = followRepo.Follow(reader.UserId, writer.UserId)

	// Пример запроса на отписку
	unfollowRequest := server.FollowRequest{
		ReaderId: reader.UserId,
		WriterId: writer.UserId,
	}

	// Создаем HTTP запрос на отписку
	unfollowJson, _ := json.Marshal(unfollowRequest)
	req := httptest.NewRequest(http.MethodPost, "/users/unfollow", bytes.NewReader(unfollowJson))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	// Выполняем запрос
	if assert.NoError(t, controller.PostUsersUnfollow(ctx)) {
		assert.Equal(t, http.StatusOK, rec.Code)

		// Проверяем, что подписка была удалена
		following, err := followRepo.GetWriters(reader.UserId)
		assert.NoError(t, err)
		assert.NotContains(t, following, writer.UserId)
	}
}
