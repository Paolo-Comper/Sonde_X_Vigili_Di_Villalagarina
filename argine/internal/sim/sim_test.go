package sim

import (
	"testing"

	"argine/internal/telemetry"
)

// Ogni sensore simulato misura sia il livello sia l'aria, in un unico payload.
func TestDefaultDevicesAreMixed(t *testing.T) {
	devices := defaultDevices()
	if len(devices) != 4 {
		t.Fatalf("sensori simulati = %d, attese 4", len(devices))
	}
	for i := range devices {
		topic, raw := devices[i].message("421342415")
		r, err := telemetry.Parse(topic, raw)
		if err != nil {
			t.Fatalf("parse %s: %v", topic, err)
		}
		if r.Kind != telemetry.KindMixed {
			t.Errorf("%s: kind = %s, atteso %s", topic, r.Kind, telemetry.KindMixed)
		}
		if r.UltrasonicCM == nil || r.TofCM == nil {
			t.Errorf("%s: mancano le misure di livello", topic)
		}
		if r.PM25 == nil || r.PressBME == nil || r.TempVirt == nil {
			t.Errorf("%s: mancano le misure aria", topic)
		}
		if r.Label == "" {
			t.Errorf("%s: label mancante", topic)
		}
	}
}
