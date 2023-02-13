package async

import (
	"context"
	"fmt"

	errorutils "k8s.io/apimachinery/pkg/util/errors"
)

type Group struct {
	ctx         context.Context
	concurrency int32
	workers     []func(stopCh chan interface{})
}

func NewGroup(ctx context.Context, concurrency int32) *Group {
	return &Group{
		ctx:         ctx,
		concurrency: concurrency,
	}
}

func (g *Group) Add(worker func(stopCh chan interface{})) {
	g.workers = append(g.workers, worker)
}

func (g *Group) Wait() error {
	if g.workers == nil || len(g.workers) == 0 {
		return nil
	}

	stopCh := make(chan interface{}, len(g.workers))
	concurCh := make(chan interface{}, g.concurrency)

	go func() {
		for _, worker := range g.workers {
			concurCh <- struct{}{}
			go worker(stopCh)
		}
	}()

	var errs []error
	res := 0
	for {
		select {
		case <-g.ctx.Done():
			return fmt.Errorf("group waiting time out")
		case val := <-stopCh:
			switch v := val.(type) {
			case error:
				errs = append(errs, v)
			case []error:
				errs = append(errs, v...)
			default:
			}
			<-concurCh
			res++
			if res == len(g.workers) {
				if len(errs) == 0 {
					return nil
				}
				return errorutils.NewAggregate(errs)
			}
		}
	}
}
