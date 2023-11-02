package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/Real-Dev-Squad/feature-flag-backend/utils"
	"github.com/aws/aws-lambda-go/events"
	lambda1 "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambda "github.com/aws/aws-sdk-go/service/lambda"
)

type Request struct {
	FunctionNames []string `json:"functionNames"`
}

type LambdaConcurrencyValue struct {
	IntValue int `json:"intValue"`
}

var createFeatureFlagFunctionName string
var getUserFeatureFlagFunctionName string
var createUserFeatureFlagFunctionName string
var getAllFeatureFlagsFunctionName string
var getUserFeatureFlagsFunctionName string
var updateFeatureFlagFunctionName string
var getFeatureFlagFunctionName string
var getUserFeatureFlagFunction string

func init() {
	env, found := os.LookupEnv(utils.ENV)
	if !found {
		log.Print("Env variable not set, making it by default PROD")
		os.Setenv(utils.ENV, utils.PROD)
	}
	log.Printf("The env is %v", env)

	createFeatureFlagFunctionName, found = os.LookupEnv("CreateFeatureFlagFunction")
	if !found {
		log.Println("Create feature flag function name not being set")
	}

	getUserFeatureFlagFunctionName, found = os.LookupEnv("GetUserFeatureFlagFunction")
	if !found {
		log.Println("Create feature flag function name not being set")
	}

	createUserFeatureFlagFunctionName, found = os.LookupEnv("CreateUserFeatureFlagFunction")
	if !found {
		log.Println("Create user feature flag function name not being set")
	}

	getUserFeatureFlagsFunctionName, found = os.LookupEnv("GetUserFeatureFlagsFunction")
	if !found {
		log.Println("get user feature flags function name not being set")
	}

	getAllFeatureFlagsFunctionName, found = os.LookupEnv("GetAllFeatureFlagFunction")
	if !found {
		log.Println("get all feature flag function name not being set")
	}

	updateFeatureFlagFunctionName, found = os.LookupEnv("UpdateFeatureFlagFunction")
	if !found {
		log.Println("Update feature flag function name not being set")
	}

	getFeatureFlagFunctionName, found = os.LookupEnv("GetFeatureFlagFunction")
	if !found {
		log.Println("get feature flag function name not being set")
	}

}

func handler(ctx context.Context, event json.RawMessage) (events.APIGatewayProxyResponse, error) {
	sess, err := session.NewSession()
	if err != nil {
		log.Println("Error in creation of AWS session, please contact on #feature-flag-service discord channel.")
	}
	lambdaClient := lambda.New(sess)

	var lambdaConcurrencyValue LambdaConcurrencyValue
	if err := json.Unmarshal(event, &lambdaConcurrencyValue); err != nil {
		return events.APIGatewayProxyResponse{
			Body: "Unable to read input",
			StatusCode: http.StatusBadRequest,
		}, nil
	}

	var request = Request{
		FunctionNames: []string{createFeatureFlagFunctionName,
			createUserFeatureFlagFunctionName,
			getFeatureFlagFunctionName,
			getUserFeatureFlagFunctionName,
			getAllFeatureFlagsFunctionName,
			updateFeatureFlagFunctionName,
			getUserFeatureFlagsFunctionName,
		},
	}

	var wg sync.WaitGroup
	for _, functionName := range request.FunctionNames {
		// Increment the WaitGroup counter
		wg.Add(1)
	
		// Start a goroutine to update the concurrency for the Lambda function
		go func(fn string) {
			defer wg.Done()
	
			input := &lambda.PutFunctionConcurrencyInput{
				FunctionName:                 &fn,
				ReservedConcurrentExecutions: aws.Int64(int64(lambdaConcurrencyValue.IntValue)),
			}
	
			log.Println("Is the function name", fn)
			_, err := lambdaClient.PutFunctionConcurrency(input)
			if err != nil {
				log.Printf("Error in setting the concurrency for the lambda name %s: %v", fn, err)
				utils.ServerError(err)
			}
	
			log.Printf("Changed the reserved concurrency for the function %s to %d", fn, lambdaConcurrencyValue.IntValue)
		}(functionName)
	}
	
	// Wait for all goroutines to finish
	wg.Wait()
	return events.APIGatewayProxyResponse{
		Body:       "Changed the reserved concurrency of the lambda function GetFeatureFlagFunction",
		StatusCode: 200,
	}, nil

}

func main() {

	lambda1.Start(handler)
}
