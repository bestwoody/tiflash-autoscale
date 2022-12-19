package autoscale

import "testing"

func TestCreateTopic(t *testing.T) {
	err := CreateTopic("autoscale_test")
	if err != nil {
		t.Errorf("[error]Create topic failed, err: %+v\n", err.Error())
		return
	}
}

func TestPublishTopology(t *testing.T) {
	err := PublishTopology("autoscale_test", "12:01", []string{"aaa", "bbb"})
	if err != nil {
		t.Errorf("[error]Publish message failed, err: %+v\n", err.Error())
		return
	}
}
