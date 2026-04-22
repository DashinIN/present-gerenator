package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const queueKey = "gen_queue"

type Task struct {
	GenerationID uuid.UUID `json:"generation_id"`
	UserID       int64     `json:"user_id"`
	EnqueuedAt   time.Time `json:"enqueued_at"`
}

type Queue struct {
	rdb *redis.Client
}

func NewQueue(rdb *redis.Client) *Queue {
	return &Queue{rdb: rdb}
}

func (q *Queue) Push(ctx context.Context, t Task) error {
	t.EnqueuedAt = time.Now()
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}
	return q.rdb.LPush(ctx, queueKey, data).Err()
}

func (q *Queue) Pop(ctx context.Context, timeout time.Duration) (*Task, error) {
	result, err := q.rdb.BRPop(ctx, timeout, queueKey).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("brpop: %w", err)
	}
	var t Task
	if err := json.Unmarshal([]byte(result[1]), &t); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}
	return &t, nil
}
