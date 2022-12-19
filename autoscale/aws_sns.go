package autoscale

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"log"
)

var (
	topicArnMap = make(map[string]string)
	region      = "us-east-2"
)

type TopologyMessage struct {
	TidbClusterID string
	Timestamp     string
	Topology      []string
}

// SNSCreateTopicAPI defines the interface for the CreateTopic function.
// We use this interface to test the function using a mocked service.
type SNSCreateTopicAPI interface {
	CreateTopic(ctx context.Context,
		params *sns.CreateTopicInput,
		optFns ...func(*sns.Options)) (*sns.CreateTopicOutput, error)
}

// SNSPublishAPI defines the interface for the Publish function.
// We use this interface to test the function using a mocked service.
type SNSPublishAPI interface {
	Publish(ctx context.Context,
		params *sns.PublishInput,
		optFns ...func(*sns.Options)) (*sns.PublishOutput, error)
}

// MakeTopic creates an Amazon Simple Notification Service (Amazon SNS) topic.
// Inputs:
//
//	c is the context of the method call, which includes the AWS Region.
//	api is the interface that defines the method call.
//	input defines the input arguments to the service call.
//
// Output:
//
//	If success, a CreateTopicOutput object containing the result of the service call and nil.
//	Otherwise, nil and an error from the call to CreateTopic.
func MakeTopic(c context.Context, api SNSCreateTopicAPI, input *sns.CreateTopicInput) (*sns.CreateTopicOutput, error) {
	return api.CreateTopic(c, input)
}

func CreateTopic(tidbClusterID string) error {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		panic("configuration error, " + err.Error())
	}
	topicName := "tiflash_cns_of_" + tidbClusterID
	client := sns.NewFromConfig(cfg)

	input := &sns.CreateTopicInput{
		Name: &topicName,
	}

	results, err := MakeTopic(context.TODO(), client, input)
	if err != nil {
		log.Printf("[error]Create topic failed, err: %+v\n", err.Error())
		return err
	}
	log.Printf("[CreateTopic]topic ARN: %v \n", *results.TopicArn)
	topicArnMap[tidbClusterID] = *results.TopicArn
	return nil
}

// PublishMessage publishes a message to an Amazon Simple Notification Service (Amazon SNS) topic
// Inputs:
//
//	c is the context of the method call, which includes the Region
//	api is the interface that defines the method call
//	input defines the input arguments to the service call.
//
// Output:
//
//	If success, a PublishOutput object containing the result of the service call and nil
//	Otherwise, nil and an error from the call to Publish
func PublishMessage(c context.Context, api SNSPublishAPI, input *sns.PublishInput) (*sns.PublishOutput, error) {
	return api.Publish(c, input)
}

func PublishTopology(tidbClusterID string, timestamp string, topology []string) error {
	topologyMessage := TopologyMessage{
		TidbClusterID: tidbClusterID,
		Timestamp:     timestamp,
		Topology:      topology,
	}
	topicARN, exist := topicArnMap[tidbClusterID]
	if !exist {
		return errors.New("The topic of " + tidbClusterID + " has not registered")
	}
	jsonTopo, err := json.Marshal(topologyMessage)
	message := string(jsonTopo)
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		panic("configuration error, " + err.Error())
	}

	client := sns.NewFromConfig(cfg)

	input := &sns.PublishInput{
		Message:  &message,
		TopicArn: &topicARN,
	}

	result, err := PublishMessage(context.TODO(), client, input)
	if err != nil {
		log.Printf("[error]Publish topology failed, err: %+v\n", err.Error())
		return err
	}
	log.Printf("[PublishTopology]message ID: %v \n", *result.MessageId)
	return nil
}
