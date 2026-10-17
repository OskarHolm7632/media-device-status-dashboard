package dashboard

import (
	"context"
	"testing"
)

type capturePublisher struct {
	snapshot DeviceSnapshot
	key      string
}

func (p *capturePublisher) Publish(_ context.Context, _, _ string, data any, key string) error {
	p.snapshot = data.(DeviceSnapshot)
	p.key = key
	return nil
}

func TestWorkflowHealthDecision(t *testing.T) {
	tests := []struct {
		name   string
		update DeviceUpdate
		health string
		reason string
	}{
		{"complete path", DeviceUpdate{"op-1", "encoder-7", "asset-9", true, "completed", "delivered", 3}, "healthy", "media path is current"},
		{"ingestion pending", DeviceUpdate{"op-2", "encoder-7", "asset-9", false, "queued", "pending", 0}, "degraded", "asset ingestion is pending"},
		{"job active", DeviceUpdate{"op-3", "encoder-7", "asset-9", true, "processing", "pending", 4}, "degraded", "processing job is not complete"},
		{"delivery pending", DeviceUpdate{"op-4", "encoder-7", "asset-9", true, "completed", "pending", 2}, "degraded", "creator delivery is pending"},
		{"queue pressure", DeviceUpdate{"op-5", "encoder-7", "asset-9", true, "completed", "delivered", 21}, "degraded", "device queue exceeds threshold"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &capturePublisher{}
			workflow := Workflow{Channel: "media-device-status", Publisher: publisher}
			got, err := workflow.Record(context.Background(), tt.update)
			if err != nil {
				t.Fatal(err)
			}
			if got.Health != tt.health || got.Reason != tt.reason {
				t.Fatalf("got %s (%s), want %s (%s)", got.Health, got.Reason, tt.health, tt.reason)
			}
			if publisher.key != tt.update.OperationID || publisher.snapshot != got {
				t.Fatal("published snapshot or idempotency key does not match decision")
			}
		})
	}
}
