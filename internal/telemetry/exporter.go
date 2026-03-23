package telemetry

import (
	"context"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Fibonacci-inspired histogram buckets for sim-domain durations (ticks/days).
var (
	DwellBuckets    = []float64{1, 2, 3, 5, 8, 13, 21}
	RecoveryBuckets = []float64{0.5, 1, 2, 3, 5, 8}
)

// Exporter bridges simulation metrics to OTel instruments.
type Exporter struct {
	snap atomic.Pointer[Snapshot]

	// Event instruments (synchronous — called from sim loop)
	deployTotal     metric.Int64Counter
	deployFailed    metric.Int64Counter
	incidentTotal   metric.Int64Counter
	leadTime        metric.Float64Histogram
	recoveryTime    metric.Float64Histogram
	dwellTime       metric.Float64Histogram
	ropeHeldTotal   metric.Int64Counter

	mu sync.Mutex // protects registration
}

// NewExporter creates an Exporter and registers all instruments on the given meter.
func NewExporter(meter metric.Meter) (*Exporter, error) {
	e := &Exporter{}

	var err error

	// Layer 2: Event instruments (synchronous counters + histograms)
	if e.deployTotal, err = meter.Int64Counter("sofdevsim_deploy_total",
		metric.WithUnit("{deploys}"),
		metric.WithDescription("Total completed deployments"),
	); err != nil {
		return nil, err
	}
	if e.deployFailed, err = meter.Int64Counter("sofdevsim_deploy_failed_total",
		metric.WithUnit("{deploys}"),
		metric.WithDescription("Total failed deployments (caused incidents)"),
	); err != nil {
		return nil, err
	}
	if e.incidentTotal, err = meter.Int64Counter("sofdevsim_incident_total",
		metric.WithUnit("{incidents}"),
		metric.WithDescription("Total incidents"),
	); err != nil {
		return nil, err
	}
	if e.leadTime, err = meter.Float64Histogram("sofdevsim_lead_time",
		metric.WithUnit("{days}"),
		metric.WithDescription("Ticket lead time distribution"),
		metric.WithExplicitBucketBoundaries(DwellBuckets...),
	); err != nil {
		return nil, err
	}
	if e.recoveryTime, err = meter.Float64Histogram("sofdevsim_incident_recovery_time",
		metric.WithUnit("{days}"),
		metric.WithDescription("Incident recovery time distribution"),
		metric.WithExplicitBucketBoundaries(RecoveryBuckets...),
	); err != nil {
		return nil, err
	}
	if e.dwellTime, err = meter.Float64Histogram("sofdevsim_phase_dwell_time",
		metric.WithUnit("{ticks}"),
		metric.WithDescription("Phase dwell time distribution"),
		metric.WithExplicitBucketBoundaries(DwellBuckets...),
	); err != nil {
		return nil, err
	}
	if e.ropeHeldTotal, err = meter.Int64Counter("sofdevsim_rope_held_total",
		metric.WithUnit("{holds}"),
		metric.WithDescription("Total tickets held at rope"),
	); err != nil {
		return nil, err
	}

	// Layer 1: State instruments (async observable gauges — read snapshot at collection time)
	if _, err = meter.Int64ObservableGauge("sofdevsim_sim_day",
		metric.WithUnit("{day}"),
		metric.WithDescription("Current simulation day"),
		metric.WithInt64Callback(e.observeSimDay),
	); err != nil {
		return nil, err
	}
	if _, err = meter.Int64ObservableGauge("sofdevsim_sim_active",
		metric.WithUnit("1"),
		metric.WithDescription("Whether simulation is active (1) or inactive (0)"),
		metric.WithInt64Callback(e.observeSimActive),
	); err != nil {
		return nil, err
	}
	if _, err = meter.Int64ObservableGauge("sofdevsim_constraint_active",
		metric.WithUnit("1"),
		metric.WithDescription("Identified constraint phase (one-hot, 1 for active constraint)"),
		metric.WithInt64Callback(e.observeConstraint),
	); err != nil {
		return nil, err
	}
	if _, err = meter.Float64ObservableGauge("sofdevsim_constraint_confidence",
		metric.WithUnit("1"),
		metric.WithDescription("Constraint identification confidence"),
		metric.WithFloat64Callback(e.observeConfidence),
	); err != nil {
		return nil, err
	}
	if _, err = meter.Int64ObservableGauge("sofdevsim_constraint_buffer_depth",
		metric.WithUnit("{tickets}"),
		metric.WithDescription("Tickets queued in front of constraint"),
		metric.WithInt64Callback(e.observeBufferDepth),
	); err != nil {
		return nil, err
	}
	if _, err = meter.Float64ObservableGauge("sofdevsim_constraint_buffer_penetration",
		metric.WithUnit("1"),
		metric.WithDescription("Constraint buffer penetration (0=full, 1=empty)"),
		metric.WithFloat64Callback(e.observeBufferPenetration),
	); err != nil {
		return nil, err
	}
	if _, err = meter.Int64ObservableGauge("sofdevsim_downstream_wip",
		metric.WithUnit("{tickets}"),
		metric.WithDescription("Aggregate WIP in rope-controlled segment"),
		metric.WithInt64Callback(e.observeDownstreamWIP),
	); err != nil {
		return nil, err
	}

	return e, nil
}

// SetSnapshot atomically updates the current snapshot for async observation.
func (e *Exporter) SetSnapshot(s *Snapshot) {
	e.snap.Store(s)
}

// Event recording methods (called from sim loop)

func (e *Exporter) RecordDeploy(ctx context.Context, simID string) {
	e.deployTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("sim_id", simID)))
}

func (e *Exporter) RecordFailedDeploy(ctx context.Context, simID string) {
	e.deployFailed.Add(ctx, 1, metric.WithAttributes(attribute.String("sim_id", simID)))
}

func (e *Exporter) RecordIncident(ctx context.Context, simID string) {
	e.incidentTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("sim_id", simID)))
}

func (e *Exporter) RecordLeadTime(ctx context.Context, simID string, days float64) {
	e.leadTime.Record(ctx, days, metric.WithAttributes(attribute.String("sim_id", simID)))
}

func (e *Exporter) RecordIncidentRecovery(ctx context.Context, simID string, days float64) {
	e.recoveryTime.Record(ctx, days, metric.WithAttributes(attribute.String("sim_id", simID)))
}

func (e *Exporter) RecordDwellTime(ctx context.Context, simID, phase string, ticks float64) {
	e.dwellTime.Record(ctx, ticks, metric.WithAttributes(
		attribute.String("sim_id", simID),
		attribute.String("phase", phase),
	))
}

func (e *Exporter) RecordRopeHeld(ctx context.Context, simID string) {
	e.ropeHeldTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("sim_id", simID)))
}

// Async observation callbacks (called at collection time by OTel SDK)

func (e *Exporter) observeSimDay(_ context.Context, o metric.Int64Observer) error {
	s := e.snap.Load()
	if s == nil || !s.Active {
		return nil
	}
	o.Observe(int64(s.SimDay), metric.WithAttributes(attribute.String("sim_id", s.SimID)))
	return nil
}

func (e *Exporter) observeSimActive(_ context.Context, o metric.Int64Observer) error {
	s := e.snap.Load()
	if s == nil {
		return nil
	}
	val := int64(0)
	if s.Active {
		val = 1
	}
	o.Observe(val, metric.WithAttributes(attribute.String("sim_id", s.SimID)))
	return nil
}

func (e *Exporter) observeConstraint(_ context.Context, o metric.Int64Observer) error {
	s := e.snap.Load()
	if s == nil || !s.Active || s.ConstraintPhase == "" {
		return nil
	}
	o.Observe(1, metric.WithAttributes(
		attribute.String("sim_id", s.SimID),
		attribute.String("phase", s.ConstraintPhase),
	))
	return nil
}

func (e *Exporter) observeConfidence(_ context.Context, o metric.Float64Observer) error {
	s := e.snap.Load()
	if s == nil || !s.Active {
		return nil
	}
	o.Observe(s.ConstraintConfidence, metric.WithAttributes(attribute.String("sim_id", s.SimID)))
	return nil
}

func (e *Exporter) observeBufferDepth(_ context.Context, o metric.Int64Observer) error {
	s := e.snap.Load()
	if s == nil || !s.Active {
		return nil
	}
	o.Observe(int64(s.BufferDepth), metric.WithAttributes(attribute.String("sim_id", s.SimID)))
	return nil
}

func (e *Exporter) observeBufferPenetration(_ context.Context, o metric.Float64Observer) error {
	s := e.snap.Load()
	if s == nil || !s.Active {
		return nil
	}
	o.Observe(s.BufferPenetration, metric.WithAttributes(attribute.String("sim_id", s.SimID)))
	return nil
}

func (e *Exporter) observeDownstreamWIP(_ context.Context, o metric.Int64Observer) error {
	s := e.snap.Load()
	if s == nil || !s.Active {
		return nil
	}
	o.Observe(int64(s.DownstreamWIP), metric.WithAttributes(attribute.String("sim_id", s.SimID)))
	return nil
}
