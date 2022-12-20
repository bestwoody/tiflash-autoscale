package autoscale

import (
	"testing"
	"time"
)

type Job interface {
	Run()
}

func TestAwsSns(t *testing.T) {
	awsSnsManager := NewAwsSnsManager("us-east-2")
	now := time.Now()
	ts := now.UnixNano()
	err := awsSnsManager.TryToPublishTopology("auto-scale", ts, []string{"a"})
	if err != nil {
		t.Errorf("[error]Create topic failed, err: %+v\n", err.Error())
		return
	}
}

func TestCurrent(t *testing.T) {
	jobQueue := make(chan string, 1000)
	awsSnsManager := NewAwsSnsManager("us-east-2")

	go func() {
		for {
			select {
			case topic := <-jobQueue:
				err := awsSnsManager.TryToPublishTopology(topic, 1, []string{"a"})
				if err != nil {
					t.Errorf("[error]Create topic failed, err: %+v\n", err.Error())
					return
				}
			}
		}
	}()
	for i := 0; i < 50; i++ {
		jobQueue <- string(rune(i))
	}
}
