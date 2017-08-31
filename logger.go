package main

import (
	"github.com/gin-gonic/gin"
	"github.com/stvp/rollbar"
)

func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				rollbar.RequestError(rollbar.ERR, c.Request, e)
			}
		}
	}
}
