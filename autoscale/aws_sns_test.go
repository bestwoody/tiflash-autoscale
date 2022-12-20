package autoscale

import "testing"

func TestAwsSns(t *testing.T) {
	awsSnsManager := NewAwsSnsManager("us-east-2")
	err := awsSnsManager.CreateTopic("autoscale_test")
	if err != nil {
		t.Errorf("[error]Create topic failed, err: %+v\n", err.Error())
		return
	}

	err = awsSnsManager.PublishTopology("autoscale_test")
	if err != nil {
		t.Errorf("[error]Publish message failed, err: %+v\n", err.Error())
		return
	}
}
