package restd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/untangle/packetd/services/logger"
	"github.com/untangle/packetd/services/reports"
	"github.com/untangle/packetd/services/settings"
	"io/ioutil"
	"strconv"
	"strings"
)

var engine *gin.Engine

// Startup is called to start the rest daemon
func Startup() {

	gin.DisableConsoleColor()
	gin.DefaultWriter = logger.NewLogWriter()
	engine = gin.Default()

	config := cors.DefaultConfig()

	// FIXME Allow cross-site for dev - this should be disabled in production
	config.AllowAllOrigins = true
	engine.Use(cors.New(config))

	// XXX should we use a server-side store instead?
	//store := cookie.NewStore([]byte(generateRandomString(32)))
	// FIXME use constant secret for now so logins persist reboots
	store := cookie.NewStore([]byte("secret"))
	engine.Use(sessions.Sessions("auth_session", store))

	engine.GET("/ping", pingHandler)

	engine.POST("/account/login", authLogin)
	//engine.GET("/account/login", authLogin)
	engine.POST("/account/logout", authLogout)
	engine.GET("/account/logout", authLogout)
	engine.GET("/account/status", authStatus)

	reports := engine.Group("/reports")
	reports.Use(authRequired(engine))
	reports.POST("/create_query", reportsCreateQuery)
	reports.GET("/get_data/:query_id", reportsGetData)

	api := engine.Group("/api")
	api.Use(authRequired(engine))
	api.GET("/settings", getSettings)
	api.GET("/settings/*path", getSettings)
	api.POST("/settings", setSettings)
	api.POST("/settings/*path", setSettings)
	api.DELETE("/settings", trimSettings)
	api.DELETE("/settings/*path", trimSettings)
	api.GET("/defaults", getDefaultSettings)
	api.GET("/defaults/*path", getDefaultSettings)

	// files
	engine.Static("/admin", "/www/admin")
	engine.Static("/static", "/www/static")

	// listen and serve on 0.0.0.0:8080
	go engine.Run()

	logger.Info("The RestD engine has been started\n")
}

// Shutdown restd
func Shutdown() {
}

func pingHandler(c *gin.Context) {
	addHeaders(c)
	c.JSON(200, gin.H{
		"message": "pong",
	})
}

func reportsGetData(c *gin.Context) {
	addHeaders(c)

	// body, err := ioutil.ReadAll(c.Request.Body)
	// if err != nil {
	// 	c.JSON(200, gin.H{"error": err})
	// 	return
	// }
	queryStr := c.Param("query_id")
	// queryID, err := strconv.ParseUint(string(body), 10, 64)
	if queryStr == "" {
		c.JSON(200, gin.H{"error": "query_id not found"})
		return
	}
	queryID, err := strconv.ParseUint(queryStr, 10, 64)
	if err != nil {
		c.JSON(200, gin.H{"error": err})
		return
	}

	str, err := reports.GetData(queryID)
	if err != nil {
		c.JSON(200, gin.H{"error": err})
		return
	}

	c.String(200, str)
	return
}

func reportsCreateQuery(c *gin.Context) {
	addHeaders(c)

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(200, gin.H{"error": err})
		return
	}
	q, err := reports.CreateQuery(string(body))
	if err != nil {
		c.JSON(200, gin.H{"error": err})
		return
	}
	str := fmt.Sprintf("%v", q.ID)
	logger.Debug("CreateQuery(%s)\n", str)
	c.String(200, str)
	// c.JSON(200, gin.H{
	// 	"queryID": q.ID,
	// })
}

func getSettings(c *gin.Context) {
	addHeaders(c)

	var segments []string

	path := c.Param("path")

	if path == "" {
		segments = nil
	} else {
		segments = removeEmptyStrings(strings.Split(path, "/"))
	}

	jsonResult := settings.GetSettings(segments)
	c.JSON(200, jsonResult)
	return
}

func getDefaultSettings(c *gin.Context) {
	addHeaders(c)

	var segments []string

	path := c.Param("path")

	if path == "" {
		segments = nil
	} else {
		segments = removeEmptyStrings(strings.Split(path, "/"))
	}

	jsonResult := settings.GetDefaultSettings(segments)
	c.JSON(200, jsonResult)
	return
}

func setSettings(c *gin.Context) {
	addHeaders(c)

	var segments []string
	path := c.Param("path")

	if path == "" {
		segments = nil
	} else {
		segments = removeEmptyStrings(strings.Split(path, "/"))
	}

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(200, gin.H{"error": err})
		return
	}
	jsonResult := settings.SetSettingsParse(segments, body)
	c.JSON(200, jsonResult)
	return
}

func trimSettings(c *gin.Context) {
	addHeaders(c)

	var segments []string
	path := c.Param("path")

	if path == "" {
		segments = nil
	} else {
		segments = removeEmptyStrings(strings.Split(path, "/"))
	}

	jsonResult := settings.TrimSettings(segments)
	c.JSON(200, jsonResult)
	return
}

func removeEmptyStrings(strings []string) []string {
	b := strings[:0]
	for _, x := range strings {
		if x != "" {
			b = append(b, x)
		}
	}
	return b
}

func addHeaders(c *gin.Context) {
	// c.Header("Example-Header", "foo")
	// c.Header("Access-Control-Allow-Origin", "*")
	// c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE")
	// c.Header("Access-Control-Allow-Headers", "X-Custom-Header")
}

func generateRandomString(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		logger.Info("Failed to generated secure key: %v\n", err)
		return "secret"
	}
	return base64.URLEncoding.EncodeToString(b)
}
