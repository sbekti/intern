//go:build integration

package auditlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/sbekti/intern/internal/db"
	"github.com/sbekti/intern/internal/testutil"
)

func TestAuditSearchAcrossPages(t *testing.T) {
	t.Parallel()
	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	service := NewService(db.New(pg.Pool))
	empty, err := service.List(ctx, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if empty.FilterOptions.Actions == nil || empty.FilterOptions.ResourceTypes == nil || len(empty.Items) != 0 {
		t.Fatalf("empty table must return empty arrays: %#v", empty)
	}
	for i := 0; i < 6; i++ {
		metadata, err := json.Marshal(map[string]any{
			"display_name": "BDO1 Camera", "mac": "AA:BB:CC:00:01", "note": "100% Office_LAN",
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = pg.Pool.Exec(ctx, `INSERT INTO audit_logs (actor_username,action,resource_type,resource_id,metadata)
			VALUES ($1,$2,$3,$4,$5)`, "Alice", "device.update", "network_device", fmt.Sprintf("device-%d", i), metadata)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = pg.Pool.Exec(ctx, `INSERT INTO audit_logs (actor_username,action,resource_type,resource_id,metadata)
		VALUES ('Bob','vlan.create','vlan','20','{}')`)
	if err != nil {
		t.Fatal(err)
	}
	type filterCase struct {
		name   string
		filter Filter
	}
	var filters []filterCase
	for _, query := range []string{" camera ", "ALIce", "ice", "DEVICE.UP", "work_dev", "device-", "aa:bb", "100%", "%", "_", "office_lan"} {
		filters = append(filters, filterCase{"q/" + query, Filter{Query: query}})
	}
	for _, actor := range []string{" ALIce ", "ice"} {
		filters = append(filters, filterCase{"actor/" + actor, Filter{ActorUsernameContains: actor}})
	}
	for _, metadata := range []string{" CAMERA ", "display_name", "aa:bb", "100%", "%", "_", "office_lan"} {
		filters = append(filters, filterCase{"metadata/" + metadata, Filter{MetadataContains: metadata}})
	}
	for _, tc := range filters {
		t.Run(tc.name, func(t *testing.T) {
			seen := map[string]bool{}
			for offset := int32(0); offset < 6; offset += 2 {
				filter := tc.filter
				filter.Limit, filter.Offset = 2, offset
				page, err := service.List(ctx, filter)
				if err != nil {
					t.Fatal(err)
				}
				if page.TotalCount != 6 || len(page.Items) != 2 {
					t.Fatalf("filter %#v: %#v", filter, page)
				}
				for _, row := range page.Items {
					seen[row.Id.String()] = true
				}
				if !reflect.DeepEqual(page.FilterOptions.Actions, []string{"device.update", "vlan.create"}) ||
					!reflect.DeepEqual(page.FilterOptions.ResourceTypes, []string{"network_device", "vlan"}) {
					t.Fatalf("options must cover all records: %#v", page.FilterOptions)
				}
			}
			if len(seen) != 6 {
				t.Fatal("search pages duplicated or lost records")
			}
		})
	}
	for _, filter := range []Filter{
		{Query: "camera", Action: "vlan.create"},
		{Query: "camera", ActorUsername: "alice"},
		{ActorUsernameContains: "Camera"},
		{MetadataContains: "Alice"},
		{MetadataContains: "100_"},
		{MetadataContains: "' OR true --"},
		{ActorUsernameContains: "ice", MetadataContains: "Guest"},
		{ActorUsernameContains: "ice", ActorUsername: "alice"},
		{MetadataContains: "Camera", Action: "vlan.create"},
		{MetadataContains: "Camera", ResourceType: "vlan"},
		{MetadataContains: "Camera", ResourceID: "device-"},
		{Query: "missing"},
		{Query: "100_"},
		{Query: "' OR true --"},
	} {
		page, err := service.List(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}
		if page.TotalCount != 0 || len(page.Items) != 0 {
			t.Fatalf("unexpected matches: %#v", filter)
		}
	}
	page, err := service.List(ctx, Filter{Query: "CAMERA", ActorUsernameContains: " ICE ", MetadataContains: "office_LAN", ResourceType: "network_device", ResourceID: "device-4", ActorUsername: "Alice", Action: "device.update"})
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 1 || len(page.Items) != 1 || page.Items[0].ResourceId != "device-4" {
		t.Fatal("search must AND with exact Activity filters")
	}
	page, err = service.List(ctx, Filter{Query: " \t ", ActorUsernameContains: " \t ", MetadataContains: " \t "})
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 7 {
		t.Fatal("blank query must not restrict results")
	}
}

func TestAuditSortAcrossPages(t *testing.T) {
	t.Parallel()
	pg := testutil.StartPostgres(t)
	ctx := context.Background()
	rows := []struct {
		actor, action, resourceType, resourceID string
		stamp                                   int
	}{
		{"bob", "device.update", "network_device", "b", 1},
		{"alice", "vlan.update", "vlan", "20", 1},
		{"alice", "device.create", "network_device", "a", 2},
		{"carol", "device.update", "gizmo", "c", 3},
		{"bob", "device.update", "network_device", "a", 3},
		{"alice", "device.update", "vlan", "10", 4},
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1)
		_, err := pg.Pool.Exec(ctx, `INSERT INTO audit_logs (id,actor_username,action,resource_type,resource_id,metadata,created_at) VALUES ($1,$2,$3,$4,$5,'{}',$6::timestamptz)`, ids[i], row.actor, row.action, row.resourceType, row.resourceID, fmt.Sprintf("2026-10-07T00:00:%02dZ", row.stamp))
		if err != nil {
			t.Fatal(err)
		}
	}
	service := NewService(db.New(pg.Pool))
	for _, tc := range []struct {
		by, dir string
		want    []int
	}{
		{"", "", []int{6, 5, 4, 3, 2, 1}},
		{"created_at", "asc", []int{1, 2, 3, 4, 5, 6}},
		{"created_at", "desc", []int{6, 5, 4, 3, 2, 1}},
		{"actor_username", "asc", []int{6, 3, 2, 5, 1, 4}},
		{"actor_username", "desc", []int{4, 5, 1, 6, 3, 2}},
		{"action", "asc", []int{3, 6, 5, 4, 1, 2}},
		{"action", "desc", []int{2, 6, 5, 4, 1, 3}},
		{"resource", "asc", []int{4, 5, 3, 1, 6, 2}},
		{"resource", "desc", []int{2, 6, 1, 5, 3, 4}},
	} {
		t.Run(tc.by+"/"+tc.dir, func(t *testing.T) {
			var got []string
			for offset := int32(0); offset < 6; offset += 2 {
				page, err := service.List(ctx, Filter{SortBy: tc.by, SortDir: tc.dir, Limit: 2, Offset: offset})
				if err != nil {
					t.Fatal(err)
				}
				if page.TotalCount != 6 || len(page.Items) != 2 {
					t.Fatalf("unexpected page: %#v", page)
				}
				for _, item := range page.Items {
					got = append(got, item.Id.String())
				}
			}
			want := make([]string, len(tc.want))
			for i, n := range tc.want {
				want[i] = ids[n-1]
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("order across pages = %v, want %v", got, want)
			}
		})
	}
	page, err := service.List(ctx, Filter{Action: "device.update", ActorUsername: "bob", SortBy: "resource", SortDir: "asc"})
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 2 || len(page.Items) != 2 || page.Items[0].Id.String() != ids[4] || page.Items[1].Id.String() != ids[0] {
		t.Fatalf("combined filter mismatch: %#v", page)
	}
	page, err = service.List(ctx, Filter{ActorUsername: "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 0 || len(page.Items) != 0 {
		t.Fatal("actor matching is no longer exact")
	}
}
