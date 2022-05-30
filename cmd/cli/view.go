package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mchmarny/dctl/pkg/data"
)

func faveIcon(c *gin.Context) {
	file, err := f.ReadFile("assets/img/favicon.ico")
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "image/x-icon", file)
}

func homeViewHandler(c *gin.Context) {
	d := gin.H{
		"version":       version,
		"err":           c.Query("err"),
		"period_months": data.EventAgeMonthsDefault,
	}

	c.HTML(http.StatusOK, "home", d)
}
