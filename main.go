package main

import (
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"go-file/common"
	"go-file/model"
	"go-file/router"
	"html/template"
	"os"
	"strconv"
	"strings"
)

func loadTemplate() *template.Template {
	var funcMap = template.FuncMap{
		"unescape": common.UnescapeHTML,
	}
	t := template.Must(template.New("").Funcs(funcMap).ParseFS(common.FS, "public/*.html"))
	return t
}

func main() {
	common.SetupGinLog()
	common.SysLog(fmt.Sprintf("Go File %s started at port %d", common.Version, *common.Port))
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	// Initialize SQL Database
	db, err := model.InitDB()
	if err != nil {
		common.FatalLog(err)
	}
	defer func(db *gorm.DB) {
		err := db.Close()
		if err != nil {
			common.FatalLog("failed to close database: " + err.Error())
		}
	}(db)

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		common.FatalLog(err)
	}

	// Initialize options
	model.InitOptionMap()

	// Initialize HTTP server
	server := gin.Default()
	server.SetHTMLTemplate(loadTemplate())

	// 打印 SessionSecret 的前几个字符（不要打印整个 secret）
	common.SysLog(fmt.Sprintf("SessionSecret (first 5 chars): %s", common.SessionSecret[:5]))

	// Initialize session store
	var store sessions.Store
	if common.RedisEnabled {
		opt := common.ParseRedisOption()
		var storeErr error
		store, storeErr = redis.NewStore(opt.MinIdleConns, opt.Network, opt.Addr, opt.Password, []byte(common.SessionSecret))
		if storeErr != nil {
			common.FatalLog(fmt.Errorf("failed to create Redis session store: %v", storeErr))
		}
		common.SysLog("Redis session store initialized successfully")
	} else {
		store = cookie.NewStore([]byte(common.SessionSecret))
		common.SysLog("Cookie session store initialized successfully")
	}

	// 检查是否使用 HTTPS
	isSecure := strings.HasPrefix(*common.Host, "https://")
	if isSecure {
		common.SysLog("HTTPS detected, session Secure option set to true")
	} else {
		common.SysLog("HTTP detected, session Secure option set to false")
	}

	// 修改 session 选项
	store.Options(sessions.Options{
		HttpOnly: true,
		MaxAge:   86400 * 7, // 7 days
		Path:     "/",
		Secure:   isSecure, // 如果使用 HTTPS，设置为 true
	})

	server.Use(sessions.Sessions("session", store))
	common.SysLog("Session middleware applied")

	router.SetRouter(server)
	var realPort = os.Getenv("PORT")
	if realPort == "" {
		realPort = strconv.Itoa(*common.Port)
	}
	if *common.Host == "" {
		ip := common.GetIp()
		if ip != "" {
			*common.Host = ip
		} else {
			*common.Host = "localhost"
		}
	}
	serverUrl := "http://" + *common.Host + ":" + realPort + "/"
	if !*common.NoBrowser {
		common.OpenBrowser(serverUrl)
	}
	if *common.EnableP2P {
		go common.StartP2PServer()
	}
	err = server.Run(":" + realPort)
	if err != nil {
		common.FatalLog(err)
	}
}
