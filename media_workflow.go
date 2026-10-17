package dashboard

import (
	"context"
	"errors"
	"fmt"
)

type DeviceUpdate struct {
	OperationID   string `json:"operation_id"`
	DeviceID      string `json:"device_id"`
	AssetID       string `json:"asset_id"`
	Ingested      bool   `json:"ingested"`
	JobState      string `json:"job_state"`
	DeliveryState string `json:"delivery_state"`
	QueueDepth    int    `json:"queue_depth"`
}

type DeviceSnapshot struct {
	DeviceID      string `json:"device_id"`
	AssetID       string `json:"asset_id"`
	Ingested      bool   `json:"ingested"`
	JobState      string `json:"job_state"`
	DeliveryState string `json:"delivery_state"`
	QueueDepth    int    `json:"queue_depth"`
	Health        string `json:"health"`
	Reason        string `json:"reason"`
}

type Publisher interface {
	Publish(context.Context, string, string, any, string) error
}

type Workflow struct {
	Channel   string
	Publisher Publisher
}

func (w Workflow) Record(ctx context.Context, update DeviceUpdate) (DeviceSnapshot, error) {
	if update.OperationID == "" || update.DeviceID == "" || update.AssetID == "" {
		return DeviceSnapshot{}, errors.New("operation_id, device_id, and asset_id are required")
	}
	if update.QueueDepth < 0 {
		return DeviceSnapshot{}, errors.New("queue_depth must be non-negative")
	}

	snapshot := DeviceSnapshot{
		DeviceID: update.DeviceID, AssetID: update.AssetID, Ingested: update.Ingested,
		JobState: update.JobState, DeliveryState: update.DeliveryState, QueueDepth: update.QueueDepth,
		Health: "healthy", Reason: "media path is current",
	}
	if !update.Ingested {
		snapshot.Health, snapshot.Reason = "degraded", "asset ingestion is pending"
	} else if update.JobState != "completed" {
		snapshot.Health, snapshot.Reason = "degraded", "processing job is not complete"
	} else if update.DeliveryState != "delivered" {
		snapshot.Health, snapshot.Reason = "degraded", "creator delivery is pending"
	} else if update.QueueDepth > 20 {
		snapshot.Health, snapshot.Reason = "degraded", "device queue exceeds threshold"
	}

	if err := w.Publisher.Publish(ctx, w.Channel, "device.status", snapshot, update.OperationID); err != nil {
		return DeviceSnapshot{}, fmt.Errorf("publish device snapshot: %w", err)
	}
	return snapshot, nil
}
