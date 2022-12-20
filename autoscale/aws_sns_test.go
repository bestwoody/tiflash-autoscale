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

//func test(awsSnsManager *AwsSnsManager, i int) {
//	log.Printf("start func: %d", i)
//	now := time.Now()
//	ts := now.UnixNano()
//	err := awsSnsManager.TryToPublishTopology(strconv.Itoa(i), ts, []string{"a"})
//	if err != nil {
//		log.Printf("[error]Create topic failed, err: %+v\n", err.Error())
//		return
//	}
//}
//
//func TestConcurrent(t *testing.T) {
//	awsSnsManager := NewAwsSnsManager("us-east-2")
//	for i := 0; i < 50; i++ {
//		go test(awsSnsManager, i)
//	}
//	time.Sleep(20 * time.Second)
//
//}
