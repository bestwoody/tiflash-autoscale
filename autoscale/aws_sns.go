package autoscale

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type AwsSnsManager struct {
	topicArnMap sync.Map
	client      *sns.Client
}

type TopologyMessage struct {
	TidbClusterID string
	Timestamp     int64
	TopologyList  []string
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

func NewAwsSnsManager(region string) (*AwsSnsManager, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	snsClient := sns.NewFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	ret := &AwsSnsManager{
		client: snsClient,
	}
	return ret, nil
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

func (c *AwsSnsManager) TryToPublishTopology(tidbClusterID string, timestamp int64, topologyList []string) error {
	topicArn, ok := c.topicArnMap.Load(tidbClusterID)
	if !ok {
		var err error
		topicArn, err = c.createTopic(tidbClusterID)
		if err != nil {
			return err
		}
	}
	return c.publishTopology(tidbClusterID, timestamp, topologyList, topicArn.(string))
}

func (c *AwsSnsManager) createTopic(tidbClusterID string) (string, error) {
	topicName := "tiflash_cns_of_" + tidbClusterID
	input := &sns.CreateTopicInput{
		Name: &topicName,
	}

	results, err := MakeTopic(context.TODO(), c.client, input)
	if err != nil {
		log.Printf("[error]Create topic failed, err: %+v\n", err.Error())
		return "", err
	}
	log.Printf("[CreateTopic]topic ARN: %v \n", *results.TopicArn)
	c.topicArnMap.Store(tidbClusterID, *results.TopicArn)
	return *results.TopicArn, nil
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

func (c *AwsSnsManager) publishTopology(tidbClusterID string, timestamp int64, topologyList []string, topicArn string) error {

	topologyMessage := TopologyMessage{
		TidbClusterID: tidbClusterID,
		Timestamp:     timestamp,
		TopologyList:  topologyList,
	}
	jsonTopo, err := json.Marshal(topologyMessage)
	if err != nil {
		log.Printf("[error][AwsSnsManager]json.Marshal(topologyMessage) fail, TiDBCluster:%v err: %v\n", tidbClusterID, err.Error())
		return err
	}
	message := string(jsonTopo)

	input := &sns.PublishInput{
		Message:  &message,
		TopicArn: &topicArn,
	}

	result, err := PublishMessage(context.TODO(), c.client, input)
	if err != nil {
		log.Printf("[error]Publish topology failed, err: %+v\n", err.Error())
		return err
	}
	log.Printf("[PublishTopology]message ID: %v \n", *result.MessageId)
	return nil
}
