package worker

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/you/fungreet/internal/models"
	"github.com/you/fungreet/internal/repository"
	"github.com/you/fungreet/internal/services"
	"golang.org/x/sync/errgroup"
)

const maxRetries = 3

type Worker struct {
	queue        *Queue
	webhookStore *WebhookStore
	genRepo      *repository.GenerationRepository
	sessionRepo  *repository.SessionRepository
	billing      *services.BillingService
	storage      services.StorageService
	imageGen     services.ImageGenerator
	songGen      services.SongGenerator
	concurrency  int
	webhookBase  string // если задан — async режим через webhook
}

func New(
	queue *Queue,
	webhookStore *WebhookStore,
	genRepo *repository.GenerationRepository,
	sessionRepo *repository.SessionRepository,
	billing *services.BillingService,
	storage services.StorageService,
	imageGen services.ImageGenerator,
	songGen services.SongGenerator,
	concurrency int,
	webhookBase string,
) *Worker {
	return &Worker{
		queue:        queue,
		webhookStore: webhookStore,
		genRepo:      genRepo,
		sessionRepo:  sessionRepo,
		billing:      billing,
		storage:      storage,
		imageGen:     imageGen,
		songGen:      songGen,
		concurrency:  concurrency,
		webhookBase:  webhookBase,
	}
}

func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := range w.concurrency {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			slog.Info("worker started", "id", id)
			w.loop(ctx, id)
		}(i)
	}
	wg.Wait()
}

func (w *Worker) loop(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		task, err := w.queue.Pop(ctx, 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("queue pop error", "worker", id, "err", err)
			continue
		}
		if task == nil {
			continue
		}

		slog.Info("processing generation", "worker", id, "generation_id", task.GenerationID)
		if err := w.process(ctx, task); err != nil {
			slog.Error("generation failed", "generation_id", task.GenerationID, "err", err)
		}
	}
}

func (w *Worker) process(ctx context.Context, task *Task) error {
	gen, err := w.genRepo.GetByID(ctx, task.GenerationID)
	if err != nil {
		return fmt.Errorf("get generation: %w", err)
	}

	fail := func(reason string) error {
		_ = w.genRepo.UpdateStatus(ctx, gen.ID, models.StatusFailed, reason)
		_ = w.billing.Refund(ctx, gen.UserID, gen.CreditsSpent, gen.ID)
		return fmt.Errorf("%s", reason)
	}

	// Async webhook режим — только submit, результат придёт по webhook
	if w.webhookBase != "" {
		return w.processAsync(ctx, gen)
	}

	// Собираем контекстные изображения из родительской генерации
	var contextImages []string
	if gen.ParentID != nil {
		parent, err := w.genRepo.GetByID(ctx, *gen.ParentID)
		if err == nil && len(parent.ResultImages) > 0 {
			contextImages = parent.ResultImages
		}
	}
	// Объединяем загруженные пользователем фото + результаты родителя как референс
	refImages := append(gen.InputPhotos, contextImages...)

	var (
		mu           sync.Mutex
		resultImages = []string{}
		resultAudios = []string{}
	)

	eg, egCtx := errgroup.WithContext(ctx)

	if gen.ImageCount > 0 {
		_ = w.genRepo.UpdateStatus(ctx, gen.ID, models.StatusProcessingImages, "")
		eg.Go(func() error {
			images, err := w.imageGen.Generate(egCtx, gen.ImagePrompt, refImages, gen.ImageCount)
			if err != nil {
				return fmt.Errorf("image generation: %w", err)
			}
			keys := make([]string, len(images))
			for i, img := range images {
				key := fmt.Sprintf("results/%s/image_%d.png", gen.ID, i)
				if err := w.storage.Upload(egCtx, key, bytes.NewReader(img), "image/png"); err != nil {
					return fmt.Errorf("upload image %d: %w", i, err)
				}
				keys[i] = key
			}
			mu.Lock()
			resultImages = keys
			mu.Unlock()
			return nil
		})
	}

	if gen.SongCount > 0 {
		if gen.ImageCount == 0 {
			_ = w.genRepo.UpdateStatus(ctx, gen.ID, models.StatusProcessingAudio, "")
		}
		eg.Go(func() error {
			lyrics := gen.SongLyrics
			if lyrics == "" && gen.SongPrompt != "" {
				if lg, ok := w.songGen.(services.LyricsGenerator); ok {
					generated, _, err := lg.GenerateLyrics(egCtx, gen.SongPrompt)
					if err != nil {
						return fmt.Errorf("lyrics generation: %w", err)
					}
					lyrics = generated
				}
			}

			uploadSongs := func(songs [][]byte, offset int) ([]string, error) {
				keys := make([]string, len(songs))
				for i, song := range songs {
					key := fmt.Sprintf("results/%s/song_%d.mp3", gen.ID, offset+i)
					if err := w.storage.Upload(egCtx, key, bytes.NewReader(song), "audio/mpeg"); err != nil {
						return nil, fmt.Errorf("upload song %d: %w", offset+i, err)
					}
					keys[i] = key
				}
				return keys, nil
			}

			if sg, ok := w.songGen.(services.StreamingSongGenerator); ok {
				var partialKeys []string
				songs, err := sg.GenerateStreaming(egCtx, lyrics, gen.SongStyle, gen.SongCount, func(partial [][]byte) {
					keys, err := uploadSongs(partial, 0)
					if err != nil {
						slog.Warn("partial upload failed", "err", err)
						return
					}
					partialKeys = keys
					if err := w.genRepo.AppendAudios(egCtx, gen.ID, keys); err != nil {
						slog.Error("AppendAudios failed", "err", err, "generation_id", gen.ID)
					} else {
						slog.Info("partial audio saved", "generation_id", gen.ID, "count", len(keys), "keys", keys)
					}
				})
				if err != nil {
					return fmt.Errorf("song generation: %w", err)
				}
				// songs содержит ВСЕ клипы (включая уже скачанные partial).
				// Загружаем только те, что ещё не были сохранены.
				newSongs := songs[len(partialKeys):]
				var finalKeys []string
				if len(newSongs) > 0 {
					keys, err := uploadSongs(newSongs, len(partialKeys))
					if err != nil {
						return err
					}
					finalKeys = keys
				}
				mu.Lock()
				resultAudios = append(partialKeys, finalKeys...)
				mu.Unlock()
			} else {
				songs, err := w.songGen.Generate(egCtx, lyrics, gen.SongStyle, gen.SongCount)
				if err != nil {
					return fmt.Errorf("song generation: %w", err)
				}
				keys, err := uploadSongs(songs, 0)
				if err != nil {
					return err
				}
				mu.Lock()
				resultAudios = keys
				mu.Unlock()
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		retries, _ := w.genRepo.IncrementRetry(ctx, gen.ID)
		if retries >= maxRetries {
			return fail(err.Error())
		}
		_ = w.genRepo.UpdateStatus(ctx, gen.ID, models.StatusPending, "")
		return w.queue.Push(ctx, *task)
	}

	if err := w.genRepo.UpdateResults(ctx, gen.ID, resultImages, resultAudios); err != nil {
		return fail(fmt.Sprintf("save results: %s", err))
	}

	// Обновляем updated_at сессии чтобы она поднялась вверх в sidebar
	if gen.SessionID != nil {
		_ = w.sessionRepo.Touch(ctx, *gen.SessionID)
	}

	slog.Info("generation completed", "generation_id", gen.ID,
		"images", len(resultImages), "songs", len(resultAudios),
		"context_images", len(contextImages))
	return nil
}

func (w *Worker) processAsync(ctx context.Context, gen *models.GenerationRequest) error {
	fail := func(reason string) error {
		_ = w.genRepo.UpdateStatus(ctx, gen.ID, models.StatusFailed, reason)
		_ = w.billing.Refund(ctx, gen.UserID, gen.CreditsSpent, gen.ID)
		return fmt.Errorf("%s", reason)
	}

	var contextImages []string
	if gen.ParentID != nil {
		if parent, err := w.genRepo.GetByID(ctx, *gen.ParentID); err == nil {
			contextImages = parent.ResultImages
		}
	}
	refImages := append(gen.InputPhotos, contextImages...)

	var pendingTypes []string

	if gen.ImageCount > 0 {
		ag, ok := w.imageGen.(services.AsyncImageGenerator)
		if !ok {
			return fail("image generator does not support async mode")
		}
		cbURL := w.webhookBase + "/api/webhooks/kie"
		taskID, err := ag.Submit(ctx, gen.ImagePrompt, refImages, cbURL)
		if err != nil {
			return fail(fmt.Sprintf("image submit: %s", err))
		}
		if err := w.webhookStore.RegisterTask(ctx, taskID, WebhookTaskMeta{
			GenID:    gen.ID.String(),
			UserID:   gen.UserID,
			TaskType: "image",
		}); err != nil {
			return fail(fmt.Sprintf("register image task: %s", err))
		}
		pendingTypes = append(pendingTypes, "image")
		slog.Info("async image submitted", "generation_id", gen.ID, "task_id", taskID)
	}

	if gen.SongCount > 0 {
		as, ok := w.songGen.(services.AsyncSongGenerator)
		if !ok {
			return fail("song generator does not support async mode")
		}
		lyrics := gen.SongLyrics
		if lyrics == "" && gen.SongPrompt != "" {
			if lg, ok := w.songGen.(services.LyricsGenerator); ok {
				generated, _, err := lg.GenerateLyrics(ctx, gen.SongPrompt)
				if err != nil {
					return fail(fmt.Sprintf("lyrics generation: %s", err))
				}
				lyrics = generated
			}
		}
		cbURL := w.webhookBase + "/api/webhooks/suno"
		taskID, err := as.Submit(ctx, lyrics, gen.SongStyle, cbURL)
		if err != nil {
			return fail(fmt.Sprintf("song submit: %s", err))
		}
		if err := w.webhookStore.RegisterTask(ctx, taskID, WebhookTaskMeta{
			GenID:    gen.ID.String(),
			UserID:   gen.UserID,
			TaskType: "song",
		}); err != nil {
			return fail(fmt.Sprintf("register song task: %s", err))
		}
		pendingTypes = append(pendingTypes, "song")
		slog.Info("async song submitted", "generation_id", gen.ID, "task_id", taskID)
	}

	if err := w.webhookStore.InitPending(ctx, gen.ID.String(), pendingTypes); err != nil {
		return fail(fmt.Sprintf("init pending: %s", err))
	}

	_ = w.genRepo.UpdateStatus(ctx, gen.ID, models.StatusProcessingImages, "")
	return nil
}
