package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const webhookTTL = 2 * time.Hour

type WebhookTaskMeta struct {
	GenID    string `json:"gen_id"`
	UserID   int64  `json:"user_id"`
	TaskType string `json:"task_type"` // "image" | "song"
}

type WebhookStore struct {
	rdb *redis.Client
}

func NewWebhookStore(rdb *redis.Client) *WebhookStore {
	return &WebhookStore{rdb: rdb}
}

func (s *WebhookStore) RegisterTask(ctx context.Context, taskID string, meta WebhookTaskMeta) error {
	data, _ := json.Marshal(meta)
	return s.rdb.Set(ctx, "wh:task:"+taskID, data, webhookTTL).Err()
}

func (s *WebhookStore) LookupTask(ctx context.Context, taskID string) (*WebhookTaskMeta, error) {
	raw, err := s.rdb.Get(ctx, "wh:task:"+taskID).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("task %s not found in webhook store", taskID)
	}
	if err != nil {
		return nil, err
	}
	var meta WebhookTaskMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// InitPending создаёт множество ожидаемых типов для генерации ({"image"}, {"song"} или {"image","song"}).
func (s *WebhookStore) InitPending(ctx context.Context, genID string, types []string) error {
	key := "wh:gen:" + genID + ":pending"
	members := make([]any, len(types))
	for i, t := range types {
		members[i] = t
	}
	pipe := s.rdb.Pipeline()
	pipe.SAdd(ctx, key, members...)
	pipe.Expire(ctx, key, webhookTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// CompletePending убирает тип из множества. Возвращает оставшееся количество.
func (s *WebhookStore) CompletePending(ctx context.Context, genID, taskType string) (int64, error) {
	key := "wh:gen:" + genID + ":pending"
	if err := s.rdb.SRem(ctx, key, taskType).Err(); err != nil {
		return 0, err
	}
	return s.rdb.SCard(ctx, key).Result()
}
