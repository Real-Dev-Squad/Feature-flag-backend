package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/Real-Dev-Squad/feature-flag-backend/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/golang-jwt/jwt/v5"
)

var (
	jwtUtilsInstance *JWTUtils
	initError        error
	once             sync.Once
)

type JWTUtils struct {
	publicKey *rsa.PublicKey
}

type EnvConfig struct {
	SessionCookieName string
	Environment       string
	PublicKeyName     string
}

func LoadEnvConfig() (*EnvConfig, error) {
	return &EnvConfig{
		SessionCookieName: os.Getenv("SESSION_COOKIE_NAME"),
		Environment:       os.Getenv("ENVIRONMENT"),
		PublicKeyName:     os.Getenv("RDS_BACKEND_PUBLIC_KEY_NAME"),
	}, nil
}

func GetInstance() (*JWTUtils, error) {
	once.Do(func() {
		jwtUtilsInstance = &JWTUtils{}
		if err := jwtUtilsInstance.initialize(); err != nil {
			initError = fmt.Errorf("internal server error")
			jwtUtilsInstance = nil
		}
	})

	if initError != nil {
		return nil, initError
	}

	if jwtUtilsInstance == nil {
		return nil, errors.New("internal server error")
	}

	return jwtUtilsInstance, nil
}

func (j *JWTUtils) initialize() error {
	if j == nil {
		return errors.New("internal server error")
	}

	envConfig, _ := LoadEnvConfig()

	parameterName := envConfig.PublicKeyName
	if parameterName == "" {
		switch envConfig.Environment {
		case utils.PROD:
			parameterName = utils.RDS_BACKEND_PUBLIC_KEY_NAME_PROD
		case utils.DEV:
			parameterName = utils.RDS_BACKEND_PUBLIC_KEY_NAME_DEV
		default:
			parameterName = utils.RDS_BACKEND_PUBLIC_KEY_NAME_LOCAL
		}
	}

	publicKeyString, err := getPublicKeyFromParameterStore(parameterName)
	if err != nil {
		return err
	}

	block, _ := pem.Decode([]byte(publicKeyString))
	if block == nil {
		return errors.New("internal server error")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("internal server error")
	}

	rsaPublicKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return errors.New("internal server error")
	}

	j.publicKey = rsaPublicKey
	return nil
}

func getPublicKeyFromParameterStore(parameterName string) (string, error) {
	sess, err := session.NewSession()
	if err != nil {
		return "", err
	}

	svc := ssm.New(sess)
	input := &ssm.GetParameterInput{
		Name:           aws.String(parameterName),
		WithDecryption: aws.Bool(true),
	}

	result, err := svc.GetParameter(input)
	if err != nil {
		return "", err
	}

	return *result.Parameter.Value, nil
}

func (j *JWTUtils) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	if j == nil || j.publicKey == nil {
		return nil, errors.New("internal server error")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("invalid token")
		}
		return j.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func (j *JWTUtils) ExtractClaim(claims jwt.MapClaims, claimKey string) (string, error) {
	if claims == nil {
		return "", errors.New("internal server error")
	}

	value, ok := claims[claimKey].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("unauthorized")
	}
	return value, nil
}

func handleMiddlewareResponse(statusCode int, message string) (events.APIGatewayProxyResponse, string, error) {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       message,
	}, "", nil
}

func JWTMiddleware() func(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, string, error) {
	return func(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, string, error) {
		jwtUtils, err := GetInstance()
		if err != nil {
			log.Printf("Failed to get JWTUtils instance: %v", err)
			return handleMiddlewareResponse(http.StatusInternalServerError, "Internal server error")
		}

		cookie := req.Headers["Cookie"]
		if cookie == "" {
			return handleMiddlewareResponse(http.StatusUnauthorized, "Unauthenticated")
		}

		envConfig, _ := LoadEnvConfig()

		cookieName := envConfig.SessionCookieName
		if cookieName == "" {
			switch envConfig.Environment {
			case utils.PROD:
				cookieName = utils.SESSION_COOKIE_NAME_PROD
			case utils.DEV:
				cookieName = utils.SESSION_COOKIE_NAME_DEV
			default:
				cookieName = utils.SESSION_COOKIE_NAME_LOCAL
			}
		}

		var jwtToken string
		cookies := strings.Split(cookie, ";")
		for _, c := range cookies {
			c = strings.TrimSpace(c)
			if strings.HasPrefix(c, cookieName+"=") {
				jwtToken = strings.TrimPrefix(c, cookieName+"=")
				break
			}
		}

		if jwtToken == "" {
			return handleMiddlewareResponse(http.StatusUnauthorized, "Unauthenticated")
		}

		claims, err := jwtUtils.ValidateToken(jwtToken)
		if err != nil {
			log.Printf("Token validation failed: %v", err)
			return handleMiddlewareResponse(http.StatusUnauthorized, "Invalid token")
		}

		userId, err := jwtUtils.ExtractClaim(claims, "userId")
		if err != nil {
			return handleMiddlewareResponse(http.StatusUnauthorized, "Unauthorized")
		}

		return handleMiddlewareResponse(http.StatusOK, userId)
	}
}
