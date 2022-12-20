package autoscale

import (
	"log"
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
	awsSnsManager := NewAwsSnsManager("us-east-2")
	for i := 0; i < 50; i++ {
		go func(i int) {
			log.Printf("start func: %dn", i)
			now := time.Now()
			ts := now.UnixNano()
			err := awsSnsManager.TryToPublishTopology(string(rune(i)), ts, []string{"a"})
			if err != nil {
				t.Errorf("[error]Create topic failed, err: %+v\n", err.Error())
				return
			}
		}(i)
	}

}
