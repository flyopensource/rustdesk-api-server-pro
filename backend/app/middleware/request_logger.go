package middleware

import (
	"fmt"
	"rustdesk-api-server-pro/config"

	"github.com/kataras/iris/v12"
)

func RequestLogger() iris.Handler {
	return func(context iris.Context) {
		if config.GetServerConfig().DebugMode && context.Path() != "/api/heartbeat" && context.Path() != "/api/device/heartbeat" {
			requestInfo := fmt.Sprintf("▶ %s:%s", context.Method(), context.Request().RequestURI)
			context.Application().Logger().Info(requestInfo)
			for header, value := range context.Request().Header {
				if header == "Authorization" {
					continue
				}
				fmt.Println(header+":", value)
			}
			if context.Path() != "/admin/auth/login" && context.Path() != "/admin/devices/profile" && context.Path() != "/admin/devices/policy" && context.Path() != "/admin/devices/server-profiles" && context.Path() != "/admin/devices/groups" {
				body, _ := context.GetBody()
				fmt.Println(string(body))
			}
		}

		context.Next()
	}
}
