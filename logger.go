package main

import (
	"github.com/gin-gonic/gin"
	"github.com/kardianos/service"
)

func loggerMiddleware(logger service.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				logger.Error(e)
			}
		}
	}
}
