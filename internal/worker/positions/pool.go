package positions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

const (
	resultQueued    = "queued"
	resultDuplicate = "duplicate"
	resultDropped   = "dropped"
	resultSuccess   = "success"
	resultError     = "error"
)

type Config struct {
	CheckInterval  time.Duration
	StaleAfter     time.Duration
	Workers        int
	QueueSize      int
	RequestTimeout time.Duration
}

type task struct {
	tripID   uuid.UUID
	driverID uuid.UUID
	token    uint64
	link     trace.Link
}

type Pool struct {
	repository    repository
	gateway       pushGateway
	metrics       metrics
	tickerFactory tickerFactory
	tracer        trace.Tracer
	log           *slog.Logger
	config        Config
	queue         chan task

	mu            sync.Mutex
	pending       map[uuid.UUID]uint64
	sequence      uint64
	started       bool
	queueClosed   bool
	scannerCancel context.CancelFunc
	scannerDone   chan struct{}
	workersCancel context.CancelFunc
	workers       sync.WaitGroup
}

func New(
	repository repository,
	gateway pushGateway,
	metrics metrics,
	tickerFactory tickerFactory,
	tracer trace.Tracer,
	log *slog.Logger,
	config Config,
) (*Pool, error) {
	if config.CheckInterval <= 0 || config.StaleAfter <= 0 || config.RequestTimeout <= 0 {
		return nil, fmt.Errorf("position worker durations must be greater than zero")
	}
	if config.Workers <= 0 || config.QueueSize <= 0 {
		return nil, fmt.Errorf("position worker sizes must be greater than zero")
	}
	return &Pool{
		repository:    repository,
		gateway:       gateway,
		metrics:       metrics,
		tickerFactory: tickerFactory,
		tracer:        tracer,
		log:           log,
		config:        config,
		queue:         make(chan task, config.QueueSize),
		pending:       make(map[uuid.UUID]uint64),
		scannerDone:   make(chan struct{}),
	}, nil
}

func (p *Pool) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return fmt.Errorf("position worker pool already started")
	}
	p.started = true
	scannerCtx, scannerCancel := context.WithCancel(ctx)
	workerCtx, workersCancel := context.WithCancel(ctx)
	p.scannerCancel = scannerCancel
	p.workersCancel = workersCancel
	p.mu.Unlock()

	for workerID := range p.config.Workers {
		p.workers.Add(1)
		go p.runWorker(workerCtx, workerID)
	}
	go p.runScanner(scannerCtx)
	return nil
}

func (p *Pool) Cancel(tripID uuid.UUID) {
	p.mu.Lock()
	delete(p.pending, tripID)
	p.mu.Unlock()
}

func (p *Pool) StopScanner() {
	p.mu.Lock()
	cancel := p.scannerCancel
	started := p.started
	p.mu.Unlock()
	if !started || cancel == nil {
		return
	}
	cancel()
	<-p.scannerDone
}

func (p *Pool) Shutdown(ctx context.Context) error {
	p.StopScanner()
	p.mu.Lock()
	if !p.queueClosed {
		close(p.queue)
		p.queueClosed = true
	}
	cancel := p.workersCancel
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.workers.Wait()
		close(done)
	}()

	select {
	case <-done:
		if cancel != nil {
			cancel()
		}
		return nil
	case <-ctx.Done():
		if cancel != nil {
			cancel()
		}
		<-done
		return fmt.Errorf("stop position workers: %w", ctx.Err())
	}
}

func (p *Pool) runScanner(ctx context.Context) {
	defer close(p.scannerDone)
	p.log.InfoContext(ctx, "position scanner started")
	defer p.log.Info("position scanner stopped")
	ticker := p.tickerFactory.NewTicker(p.config.CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			p.scan(ctx)
		}
	}
}

func (p *Pool) scan(ctx context.Context) {
	ctx, span := p.tracer.Start(ctx, "positions.scan")
	defer span.End()
	trips, err := p.repository.ListStaleTrips(ctx, p.config.StaleAfter)
	if err != nil {
		span.RecordError(err)
		p.log.ErrorContext(ctx, "stale trips scan failed", "err", err)
		return
	}
	link := trace.Link{SpanContext: span.SpanContext()}
	for _, staleTrip := range trips {
		p.enqueue(ctx, staleTrip, link)
	}
}

func (p *Pool) enqueue(ctx context.Context, staleTrip domaintrip.StaleTrip, link trace.Link) {
	p.mu.Lock()
	if _, exists := p.pending[staleTrip.TripID]; exists {
		p.mu.Unlock()
		p.metrics.StaleCheck(ctx, resultDuplicate)
		return
	}
	p.sequence++
	task := task{tripID: staleTrip.TripID, driverID: staleTrip.DriverID, token: p.sequence, link: link}
	p.pending[staleTrip.TripID] = task.token
	select {
	case p.queue <- task:
		queueSize := len(p.queue)
		p.mu.Unlock()
		p.metrics.SetPositionQueueSize(int64(queueSize))
		p.metrics.StaleCheck(ctx, resultQueued)
		return
	default:
		delete(p.pending, staleTrip.TripID)
		p.mu.Unlock()
		p.metrics.StaleCheck(ctx, resultDropped)
		p.metrics.PositionRequestDropped(ctx)
		p.log.WarnContext(ctx, "position request dropped", "trip_id", staleTrip.TripID)
		return
	}
}

func (p *Pool) runWorker(ctx context.Context, workerID int) {
	defer p.workers.Done()
	p.log.InfoContext(ctx, "position worker started", "worker_id", workerID)
	defer p.log.Info("position worker stopped", "worker_id", workerID)
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-p.queue:
			if !ok {
				return
			}
			p.metrics.SetPositionQueueSize(int64(len(p.queue)))
			if !p.isPending(task) {
				continue
			}
			p.process(ctx, task)
			p.complete(task)
		}
	}
}

func (p *Pool) isPending(task task) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pending[task.tripID] == task.token
}

func (p *Pool) complete(task task) {
	p.mu.Lock()
	if p.pending[task.tripID] == task.token {
		delete(p.pending, task.tripID)
	}
	p.mu.Unlock()
}

func (p *Pool) process(ctx context.Context, task task) {
	ctx, span := p.tracer.Start(
		ctx,
		"positions.request",
		trace.WithNewRoot(),
		trace.WithLinks(task.link),
		trace.WithAttributes(attribute.String("trip.id", task.tripID.String())),
	)
	defer span.End()
	p.metrics.PositionWorkerStarted(ctx)
	defer p.metrics.PositionWorkerFinished(ctx)

	requestCtx, cancel := context.WithTimeout(ctx, p.config.RequestTimeout)
	startedAt := time.Now()
	err := p.gateway.RequestPosition(requestCtx, task.tripID, task.driverID)
	cancel()
	result := resultSuccess
	if err != nil {
		result = resultError
		span.RecordError(err)
		p.log.WarnContext(ctx, "position request failed", "trip_id", task.tripID, "err", err)
	}
	p.metrics.PositionRequest(ctx, result, time.Since(startedAt))
}
