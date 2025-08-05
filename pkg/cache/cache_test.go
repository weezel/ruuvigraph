package cache

import (
	"sync"
	"testing"
	"time"

	ruuviv1 "weezel/ruuvigraph/pkg/generated/ruuvi/ruuvi/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMeasurements_pruneOldData(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("Run with `test -race` and without -short")
	}

	started := time.Now()
	m := New(WithTickerRate(time.Second*1), WithMaxMeasureAge(time.Second*2))

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 5000 {
			switch {
			case i%2 == 0:
				m.Add(&ruuviv1.RuuviStreamDataRequest{
					Timestamp: timestamppb.New(time.Now().Add(time.Second - 2)),
				})
			case i%3 == 0:
				m.Add(&ruuviv1.RuuviStreamDataRequest{
					Timestamp: timestamppb.New(time.Now().Add(time.Second - 3)),
				})
			default:
				m.Add(&ruuviv1.RuuviStreamDataRequest{
					Timestamp: timestamppb.New(time.Now().Add(time.Second - 1)),
				})
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 5000 {
			switch {
			case i%2 == 0:
				m.Add(&ruuviv1.RuuviStreamDataRequest{
					Timestamp: timestamppb.New(time.Now().Add(time.Second - 2)),
				})
			case i%3 == 0:
				m.Add(&ruuviv1.RuuviStreamDataRequest{
					Timestamp: timestamppb.New(time.Now().Add(time.Second - 3)),
				})
			default:
				m.Add(&ruuviv1.RuuviStreamDataRequest{
					Timestamp: timestamppb.New(time.Now().Add(time.Second - 1)),
				})
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 20 {
			time.Sleep(time.Millisecond * 50)
			m.pruneOldData()
		}
	}()

	wg.Wait()

	t.Logf("Got %d items in cache", len(m.All()))
	for l := len(m.All()); l != 0; l = len(m.All()) {
		t.Log("Not yet, sleeping...")
		time.Sleep(time.Second * 1)
	}
	t.Logf("Took %s", time.Since(started))
}
