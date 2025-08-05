package component

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	nebula0 "github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestCheckPeerExist(t *testing.T) {
	tests := []struct {
		name         string
		failureHosts []string
		spaceItems   map[int32][]*meta.PartItem
		want         bool
	}{
		{
			name:         "Multiple failure hosts in same partition",
			failureHosts: []string{"host1", "host2", "host5", "host9"},
			spaceItems: map[int32][]*meta.PartItem{
				1: {
					{PartID: 0, Peers: []*nebula0.HostAddr{{Host: "host1"}, {Host: "host2"}, {Host: "host3"}}},
					{PartID: 1, Peers: []*nebula0.HostAddr{{Host: "host4"}, {Host: "host5"}, {Host: "host6"}}},
					{PartID: 2, Peers: []*nebula0.HostAddr{{Host: "host7"}, {Host: "host8"}, {Host: "host9"}}},
				},
			},
			want: true,
		},
		{
			name:         "Single failure host in each partition",
			failureHosts: []string{"host1", "host4", "host10"},
			spaceItems: map[int32][]*meta.PartItem{
				1: {
					{PartID: 0, Peers: []*nebula0.HostAddr{{Host: "host1"}, {Host: "host2"}, {Host: "host3"}}},
					{PartID: 1, Peers: []*nebula0.HostAddr{{Host: "host4"}, {Host: "host5"}, {Host: "host6"}}},
					{PartID: 2, Peers: []*nebula0.HostAddr{{Host: "host8"}, {Host: "host9"}, {Host: "host10"}}},
				},
			},
			want: false,
		},
		{
			name:         "Failure hosts across multiple spaces",
			failureHosts: []string{"host1", "host2", "host3", "host4"},
			spaceItems: map[int32][]*meta.PartItem{
				1: {
					{PartID: 0, Peers: []*nebula0.HostAddr{{Host: "host1"}, {Host: "host4"}, {Host: "host5"}}},
				},
				2: {
					{PartID: 0, Peers: []*nebula0.HostAddr{{Host: "host2"}, {Host: "host3"}, {Host: "host6"}}},
				},
			},
			want: true,
		},
		{
			name:         "Different failure hosts in different spaces",
			failureHosts: []string{"host1", "host4", "host7", "host10"},
			spaceItems: map[int32][]*meta.PartItem{
				1: {
					{PartID: 0, Peers: []*nebula0.HostAddr{{Host: "host1"}, {Host: "host2"}, {Host: "host3"}}},
					{PartID: 1, Peers: []*nebula0.HostAddr{{Host: "host4"}, {Host: "host5"}, {Host: "host6"}}},
				},
				2: {
					{PartID: 0, Peers: []*nebula0.HostAddr{{Host: "host7"}, {Host: "host8"}, {Host: "host9"}}},
					{PartID: 1, Peers: []*nebula0.HostAddr{{Host: "host10"}, {Host: "host11"}, {Host: "host12"}}},
				},
			},
			want: false,
		},
		{
			name:         "Edge case-empty lists",
			failureHosts: []string{},
			spaceItems:   map[int32][]*meta.PartItem{},
			want:         false,
		},
		{
			name:         "Edge case-empty failure hosts with non-empty space items",
			failureHosts: []string{},
			spaceItems: map[int32][]*meta.PartItem{
				1: {
					{PartID: 0, Peers: []*nebula0.HostAddr{{Host: "host1"}, {Host: "host2"}, {Host: "host3"}}},
				},
			},
			want: false,
		},
		{
			name:         "Edge case-non-empty failure hosts with empty space items",
			failureHosts: []string{"host1", "host2"},
			spaceItems:   map[int32][]*meta.PartItem{},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failureHostsSet := sets.NewString(tt.failureHosts...)
			var foundResult int32
			resultCh := make(chan struct {
				found bool
				err   error
			}, 1)

			_, cancel := context.WithCancel(context.Background())
			defer cancel()

			var wg sync.WaitGroup
			sem := make(chan struct{}, 10)

			for spaceID, parts := range tt.spaceItems {
				for _, part := range parts {
					wg.Add(1)
					sem <- struct{}{}
					go func(spaceID int32, part *meta.PartItem) {
						defer wg.Done()
						defer func() { <-sem }()

						if atomic.LoadInt32(&foundResult) == 1 {
							return
						}

						peers := convertNebulaHostAddr(part.Peers)
						peerSet := sets.NewString(peers...)
						interSection := peerSet.Intersection(failureHostsSet)

						if interSection.Len() >= 2 {
							if atomic.CompareAndSwapInt32(&foundResult, 0, 1) {
								t.Logf("space %d part %d has more than 2 failure hosts: %v",
									spaceID, part.PartID, interSection.List())

								cancel()
								resultCh <- struct {
									found bool
									err   error
								}{true, nil}
							}
						}
					}(spaceID, part)
				}
			}

			go func() {
				wg.Wait()
				close(resultCh)
			}()

			found := false
			for result := range resultCh {
				if result.err != nil {
					t.Errorf("unexpected error: %v", result.err)
				}
				if result.found {
					found = true
					break
				}
			}

			if found != tt.want {
				t.Errorf("checkPeerExist() = %v, want %v", found, tt.want)
			}
		})
	}
}
