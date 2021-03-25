package middleware

import (
	"go-xops/internal/request"
	"go-xops/internal/response"
	"go-xops/internal/service"
	"go-xops/pkg/cache"
	"go-xops/pkg/common"
	"go-xops/pkg/utils"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var loginInfo response.LoginResp

func InitAuth() (*jwt.GinJWTMiddleware, error) {
	return jwt.New(&jwt.GinJWTMiddleware{
		Realm:            common.Conf.Jwt.Realm,
		SigningAlgorithm: "HS512",
		Key:              []byte(common.Conf.Jwt.Key),
		Timeout:          time.Hour * time.Duration(common.Conf.Jwt.Timeout),
		MaxRefresh:       time.Hour * time.Duration(common.Conf.Jwt.MaxRefresh),
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(map[string]interface{}); ok {
				var user response.LoginResp
				utils.JsonI2Struct(v["user"], &user)
				return jwt.MapClaims{
					jwt.IdentityKey: user.Id,
					"user":          v["user"],
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return map[string]interface{}{
				"IdentityKey": claims[jwt.IdentityKey],
				"user":        claims["user"],
			}
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var req request.RegisterAndLoginReq
			_ = c.ShouldBindJSON(&req)
			s := service.New()
			user, err := s.LoginCheck(req.Username, req.Password)
			if err != nil {
				return nil, err
			}
			loginInfo = *user
			ma := map[string]interface{}{
				"user": utils.Struct2Json(user),
			}
			return ma, nil
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			if v, ok := data.(map[string]interface{}); ok {
				var user response.LoginResp
				utils.JsonI2Struct(v["user"], &user)
				c.Set("user", user)
				return true
			}
			return false
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			if message == response.LoginCheckErrorMsg {
				response.FailWithMsg(response.LoginCheckErrorMsg)
				return
			} else if message == response.UserForbiddenMsg {
				response.FailWithCode(response.UserForbidden)
				return
			}

			response.FailWithCode(response.Unauthorized)
		},
		LoginResponse: func(c *gin.Context, code int, token string, expires time.Time) {
			cache, err := cache.New(time.Duration(common.Conf.Jwt.Timeout))
			if err != nil {
				logrus.Error(err)
			}
			tokenKey := "token:" + loginInfo.Username
			expiresKey := "expires:" + loginInfo.Username
			cacheToken, _ := cache.Get(tokenKey)
			cacheExpires, _ := cache.Get(expiresKey)
			if cacheToken == nil {
				cacheToken = token
				cache.Set(tokenKey, cacheToken)
			}

			if cacheExpires == nil {
				cacheExpires = expires.Format("2006-01-02 15:04:05")
				cache.Set(expiresKey, cacheExpires)
			}
			loginInfo.Token = cacheToken.(string)
			loginInfo.Expires = cacheExpires.(string)
			response.SuccessWithData(loginInfo)
		},
		LogoutResponse: func(c *gin.Context, code int) {
			response.Success()
		},
		TokenLookup:   "header: Authorization, query: token",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	})
}
