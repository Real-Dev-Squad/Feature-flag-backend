package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Real-Dev-Squad/feature-flag-backend/database"
	"github.com/Real-Dev-Squad/feature-flag-backend/models"
	"github.com/Real-Dev-Squad/feature-flag-backend/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

func processGetById(userId string, flagId string) (*models.FeatureFlagUserMapping, error) {

	db := database.CreateDynamoDB()

	input := &dynamodb.GetItemInput{
		TableName: aws.String(database.GetTableName(utils.FFUM_TABLE_NAME)),
		Key: map[string]*dynamodb.AttributeValue{
			utils.UserId: {
				S: aws.String(userId),
			},
			utils.FlagId: {
				S: aws.String(flagId),
			},
		},
	}

	result, err := db.GetItem(input)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	if result.Item == nil {
		return nil, nil
	}
	featureFlagUserMapping := new(models.FeatureFlagUserMapping)
	err = dynamodbattribute.UnmarshalMap(result.Item, &featureFlagUserMapping)

	if err != nil {
		log.Println(err)
		return nil, err
	}
	return featureFlagUserMapping, nil
}

func handler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	userId, found := req.PathParameters["userId"]
	if !found {
		log.Panic("User ID not passed")
		return utils.ClientError(http.StatusBadRequest, "User ID not passed in request")
	}

	flagId, found := req.PathParameters["flagId"]
	if !found {
		log.Panic("Flag ID not passed")
		return utils.ClientError(http.StatusBadRequest, "Flag ID not passed in request")
	}

	result, err := processGetById(userId, flagId)
	if err != nil {
		return utils.ServerError(err)
	}
	if result == nil {
		log.Println("User feature flag not found")
		return utils.ClientError(http.StatusNotFound, "User feature flag not found")
	}
	resultJson, err := json.Marshal(result)
	if err != nil {
		log.Println("Error converting featureFlagUserMapping to JSON")
		return utils.ServerError(err)
	}

	return events.APIGatewayProxyResponse{
		Body:       string(resultJson),
		StatusCode: http.StatusOK,
	}, nil

}

func main() {
	lambda.Start(handler)
}
