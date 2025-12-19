// middleware/google_oauth.go
package middlewares

import (
	"chat-backend/config"
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"

	"github.com/gin-gonic/gin"
)

func GoogleOAuthLogin(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		url := cfg.GoogleOAuthConfig.AuthCodeURL("random-state-string", oauth2.AccessTypeOffline)
		c.Redirect(http.StatusTemporaryRedirect, url)
	}
}

func GoogleOAuthCallback(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Code not provided"})
			return
		}

		// Exchange the authorization code for a token.
		token, err := cfg.GoogleOAuthConfig.Exchange(context.TODO(), code)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to exchange token"})
			return
		}

		// Create a new OAuth2 client using the token
		client := cfg.GoogleOAuthConfig.Client(context.TODO(), token)

		// Create a new OAuth2 service using the client
		oauthService, err := oauth2.NewService(context.TODO(), option.WithHTTPClient(client))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating OAuth service"})
			return
		}

		userInfo, err := oauthService.Userinfo.Get().Do()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}

		// Handle user information, e.g., save to database
		fmt.Printf("User Info: %#v\n", userInfo)

		// Set user session or JWT here
		c.JSON(http.StatusOK, gin.H{
			"email":   userInfo.Email,
			"picture": userInfo.Picture,
			"name":    userInfo.Name,
		})
	}
}
