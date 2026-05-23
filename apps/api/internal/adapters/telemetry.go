package adapters

import (
	"log"

	"github.com/nydhy/reclaimo/apps/api/internal/events"
)

type LogTelemetrySink struct {
	service string
}

func NewLogTelemetrySink(service string) *LogTelemetrySink {
	if service == "" {
		service = "reclaimo-api"
	}
	return &LogTelemetrySink{service: service}
}

func (s *LogTelemetrySink) Write(event events.Event) error {
	log.Printf("telemetry service=%s event=%s id=%s", s.service, event.Type, event.ID)
	return nil
}
