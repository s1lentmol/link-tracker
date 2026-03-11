package domain

import "time"

type Subscription struct {
	ID      int64
	URL     string
	Tags    []string
	Filters []string
}

type Resource struct {
	ID         int64
	URL        string
	LastUpdate time.Time
	ChatIDs    []int64
}
