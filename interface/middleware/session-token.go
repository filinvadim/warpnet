package middleware

import (
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"net/http"
	"strings"
	"sync"
)

type SessionTokenMiddleware struct {
	mx    *sync.RWMutex
	token string
}

func NewSessionTokenMiddleware() *SessionTokenMiddleware {
	return &SessionTokenMiddleware{
		mx:    new(sync.RWMutex),
		token: "",
	}
}

func (m *SessionTokenMiddleware) VerifySessionToken(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.Request() == nil {
			return next(c)
		}
		if c.Request().URL == nil {
			return next(c)
		}
		if c.Request().URL.Path == "/" {
			return next(c)
		}
		if !strings.Contains(c.Request().URL.Path, "/login") {
			sessionToken := c.Request().Header.Get("X-SESSION-TOKEN")
			if sessionToken == "" {
				log.Error("missing X-SESSION-TOKEN header")
				return c.JSON(http.StatusBadRequest, errors.New("missing X-SESSION-TOKEN header"))
			}
			m.mx.RLock()
			isTokenValid := sessionToken == m.token
			m.mx.RUnlock()

			if !isTokenValid {
				log.Error("invalid session token")
				return c.JSON(http.StatusBadRequest, errors.New("invalid session token"))
			}
		}

		if err := next(c); err != nil {
			return c.JSON(http.StatusBadRequest, fmt.Errorf("middleware next: %w", err))
		}
		if strings.Contains(c.Request().URL.Path, "/login") {
			m.mx.Lock()
			m.token = c.Response().Header().Get("X-SESSION-TOKEN")
			m.mx.Unlock()
		}
		return nil
	}
}
