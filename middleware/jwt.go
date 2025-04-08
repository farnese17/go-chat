package middleware

import (
	"strings"
	"time"

	"github.com/farnese17/chat/config"
	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var mySigningKey = []byte("go-chat signing")
var invalidToken = errorsx.ErrInvalidToken.Error()

type MyClaim struct {
	ID uint
	jwt.RegisteredClaims
}

func GenerateToken(id uint) (string, error) {
	expire := config.GetConfig().Common().TokenValidPeriod()
	claims := MyClaim{
		ID: id,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "go-chat",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(mySigningKey)
}

func ParseToken(tokenStr string) (*MyClaim, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &MyClaim{}, func(t *jwt.Token) (interface{}, error) {
		return mySigningKey, nil
	})
	if err != nil {
		return nil, err
	}
	if chaims, ok := token.Claims.(*MyClaim); ok && token.Valid {
		return chaims, nil
	}
	return nil, errorsx.ErrInvalidToken
}

func JWT(status int) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Request.Header.Get("Authorization")
		pre := "Bearer "
		if !strings.HasPrefix(token, pre) {
			c.JSON(status, gin.H{
				"status":  errorsx.GetStatusCode(errorsx.ErrInvalidToken),
				"message": invalidToken,
			})
			c.Abort()
			return
		}
		chaim, err := ParseToken(token[len(pre):])
		if err != nil {
			c.JSON(status, gin.H{
				"status":  errorsx.GetStatusCode(errorsx.ErrInvalidToken),
				"message": invalidToken,
			})
			c.Abort()
			return
		}
		c.Set("from", chaim.ID)
		c.Next()
	}
}

func VerifyTokenInWhitelist(status int) gin.HandlerFunc {
	cache := registry.GetService().Cache()
	return func(c *gin.Context) {
		token := c.Request.Header.Get("Authorization")
		pre := "Bearer "
		token = token[len(pre):]

		id := c.MustGet("from").(uint)
		val, err := cache.GetToken(id)
		if err != nil {
			c.JSON(status, gin.H{
				"status":  errorsx.GetStatusCode(errorsx.ErrCantParseToken),
				"message": errorsx.ErrCantParseToken.Error(),
			})
			c.Abort()
			return
		}
		if val != token {
			c.JSON(status, gin.H{
				"status":  errorsx.GetStatusCode(errorsx.ErrInvalidToken),
				"message": invalidToken,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
