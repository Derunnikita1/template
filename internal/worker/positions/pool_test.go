package positions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type manualTicker struct {
	ch      chan time.Time
	stopped chan struct{}
	once    sync.Once
}

func newManualTicker() *manualTicker {
	return &manualTicker{ch: make(chan time.Time), stopped: make(chan struct{})}
}

func (t *manualTicker) C() <-chan time.Time {
	return t.ch
}

func (t *manualTicker) Stop() {
	t.once.Do(func() { close(t.stopped) })
}

type manualTickerFactory struct {
	ticker *manualTicker
}

func (f manualTickerFactory) NewTicker(time.Duration) ticker {
	return f.ticker
}

type metricsRecorder struct {
	results chan string
}

func newMetricsRecorder() *metricsRecorder {
	return &metricsRecorder{results: make(chan string, 128)}
}

func (m *metricsRecorder) StaleCheck(_ context.Context, result string) {
	m.results <- result
}

func (m *metricsRecorder) PositionRequestDropped(context.Context) {}

func (m *metricsRecorder) PositionRequest(context.Context, string, time.Duration) {}

func (m *metricsRecorder) SetPositionQueueSize(int64) {}

func (m *metricsRecorder) PositionWorkerStarted(context.Context) {}

func (m *metricsRecorder) PositionWorkerFinished(context.Context) {}

func newTestPool(
	t *testing.T,
	repository repository,
	gateway pushGateway,
	metrics metrics,
	tickerFactory tickerFactory,
	workers int,
	queueSize int,
) *Pool {
	t.Helper()
	pool, err := New(
		repository,
		gateway,
		metrics,
		tickerFactory,
		otel.Tracer("test"),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Config{
			CheckInterval:  time.Second,
			StaleAfter:     time.Minute,
			Workers:        workers,
			QueueSize:      queueSize,
			RequestTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

func shutdownPool(t *testing.T, pool *Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := pool.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestScannerDoesNotQueueDuplicateOnNextTick(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMockrepository(ctrl)
	gateway := NewMockpushGateway(ctrl)
	ticker := newManualTicker()
	metrics := newMetricsRecorder()
	tripID := uuid.New()
	driverID := uuid.New()
	stale := []domaintrip.StaleTrip{{TripID: tripID, DriverID: driverID}}
	repository.EXPECT().ListStaleTrips(gomock.Any(), time.Minute).Return(stale, nil).Times(2)
	started := make(chan struct{})
	release := make(chan struct{})
	gateway.EXPECT().RequestPosition(gomock.Any(), tripID, driverID).DoAndReturn(
		func(context.Context, uuid.UUID, uuid.UUID) error {
			close(started)
			<-release
			return nil
		},
	)
	pool := newTestPool(t, repository, gateway, metrics, manualTickerFactory{ticker}, 1, 2)
	if err := pool.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ticker.ch <- time.Now()
	if result := <-metrics.results; result != resultQueued {
		t.Fatalf("first tick result = %s", result)
	}
	<-started
	ticker.ch <- time.Now()
	if result := <-metrics.results; result != resultDuplicate {
		t.Fatalf("second tick result = %s", result)
	}
	close(release)
	shutdownPool(t, pool)
}

func TestCoordinateCancelsQueuedRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMockrepository(ctrl)
	gateway := NewMockpushGateway(ctrl)
	metrics := newMetricsRecorder()
	ticker := newManualTicker()
	firstTrip := domaintrip.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}
	secondTrip := domaintrip.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}
	started := make(chan struct{})
	release := make(chan struct{})
	gateway.EXPECT().RequestPosition(gomock.Any(), firstTrip.TripID, firstTrip.DriverID).DoAndReturn(
		func(context.Context, uuid.UUID, uuid.UUID) error {
			close(started)
			<-release
			return nil
		},
	)
	pool := newTestPool(t, repository, gateway, metrics, manualTickerFactory{ticker}, 1, 2)
	pool.enqueue(context.Background(), firstTrip, trace.Link{})
	if err := pool.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	<-started
	pool.enqueue(context.Background(), secondTrip, trace.Link{})
	pool.Cancel(secondTrip.TripID)
	close(release)
	shutdownPool(t, pool)
}

func TestFullQueueDropsWithoutBlockingScanner(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMockrepository(ctrl)
	gateway := NewMockpushGateway(ctrl)
	metrics := newMetricsRecorder()
	ticker := newManualTicker()
	blocked := domaintrip.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}
	queued := domaintrip.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}
	dropped := domaintrip.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}
	started := make(chan struct{})
	release := make(chan struct{})
	gateway.EXPECT().RequestPosition(gomock.Any(), blocked.TripID, blocked.DriverID).DoAndReturn(
		func(context.Context, uuid.UUID, uuid.UUID) error {
			close(started)
			<-release
			return nil
		},
	)
	gateway.EXPECT().RequestPosition(gomock.Any(), queued.TripID, queued.DriverID).Return(nil)
	repository.EXPECT().ListStaleTrips(gomock.Any(), time.Minute).Return(
		[]domaintrip.StaleTrip{queued, dropped}, nil,
	)
	pool := newTestPool(t, repository, gateway, metrics, manualTickerFactory{ticker}, 1, 1)
	pool.enqueue(context.Background(), blocked, trace.Link{})
	<-metrics.results
	if err := pool.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	<-started
	ticker.ch <- time.Now()
	if result := <-metrics.results; result != resultQueued {
		t.Fatalf("queued result = %s", result)
	}
	if result := <-metrics.results; result != resultDropped {
		t.Fatalf("overflow result = %s", result)
	}
	close(release)
	shutdownPool(t, pool)
}

func TestPoolProcessesTasksWithConfiguredWorkers(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMockrepository(ctrl)
	gateway := NewMockpushGateway(ctrl)
	metrics := newMetricsRecorder()
	ticker := newManualTicker()
	const workerCount = 4
	const taskCount = 8
	started := make(chan struct{}, taskCount)
	release := make(chan struct{})
	var active atomic.Int32
	var maximum atomic.Int32
	gateway.EXPECT().RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, uuid.UUID, uuid.UUID) error {
			current := active.Add(1)
			for {
				previous := maximum.Load()
				if current <= previous || maximum.CompareAndSwap(previous, current) {
					break
				}
			}
			started <- struct{}{}
			<-release
			active.Add(-1)
			return nil
		},
	).Times(taskCount)
	pool := newTestPool(t, repository, gateway, metrics, manualTickerFactory{ticker}, workerCount, taskCount)
	for range taskCount {
		pool.enqueue(context.Background(), domaintrip.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}, trace.Link{})
	}
	if err := pool.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	for range workerCount {
		<-started
	}
	if maximum.Load() != workerCount {
		t.Fatalf("maximum active workers = %d", maximum.Load())
	}
	close(release)
	shutdownPool(t, pool)
}

func TestContextCancellationStopsScannerAndWorkers(t *testing.T) {
	ctrl := gomock.NewController(t)
	pool := newTestPool(
		t,
		NewMockrepository(ctrl),
		NewMockpushGateway(ctrl),
		newMetricsRecorder(),
		manualTickerFactory{newManualTicker()},
		2,
		2,
	)
	ctx, cancel := context.WithCancel(context.Background())
	if err := pool.Start(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-pool.scannerDone:
	case <-time.After(time.Second):
		t.Fatal("scanner did not stop")
	}
	done := make(chan struct{})
	go func() {
		pool.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("workers did not stop")
	}
	shutdownPool(t, pool)
}

func TestPushErrorDoesNotStopWorker(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMockrepository(ctrl)
	gateway := NewMockpushGateway(ctrl)
	metrics := newMetricsRecorder()
	ticker := newManualTicker()
	first := domaintrip.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}
	second := domaintrip.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}
	processed := make(chan struct{})
	gomock.InOrder(
		gateway.EXPECT().RequestPosition(gomock.Any(), first.TripID, first.DriverID).Return(errors.New("push unavailable")),
		gateway.EXPECT().RequestPosition(gomock.Any(), second.TripID, second.DriverID).DoAndReturn(
			func(context.Context, uuid.UUID, uuid.UUID) error {
				close(processed)
				return nil
			},
		),
	)
	pool := newTestPool(t, repository, gateway, metrics, manualTickerFactory{ticker}, 1, 2)
	pool.enqueue(context.Background(), first, trace.Link{})
	pool.enqueue(context.Background(), second, trace.Link{})
	if err := pool.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-processed:
	case <-time.After(time.Second):
		t.Fatal("second task was not processed")
	}
	shutdownPool(t, pool)
}
