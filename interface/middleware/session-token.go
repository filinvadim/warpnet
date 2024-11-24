package middleware

import (
	"errors"
	"github.com/labstack/echo/v4"
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
		if !strings.Contains(c.Request().URL.Path, "login") {
			sessionToken := c.Request().Header.Get("X-SESSION-TOKEN")
			if sessionToken == "" {
				c.Error(errors.New("missing X-SESSION-TOKEN header"))
			}
			if sessionToken != m.token {
				c.Error(errors.New("invalid session token"))
			}
		}

		if err := next(c); err != nil {
			c.Error(err)
		}
		if strings.Contains(c.Request().URL.Path, "login") {
			m.token = c.Response().Header().Get("X-SESSION-TOKEN")
		}
		return nil
	}
}
