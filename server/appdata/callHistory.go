package appdata

import (
	"sync"
	"time"
)

// CallRecord captures a single incoming HTTP call and which mapping (if any)
// was used to generate the response.
type CallRecord struct {
	Time        time.Time           `json:"time"`
	Method      string              `json:"method"`
	URL         string              `json:"url"`
	Query       map[string][]string `json:"query,omitempty"`
	RequestBody any                 `json:"requestBody,omitempty"`
	MappingID   string              `json:"mappingId,omitempty"`
	Status      int                 `json:"status"`
}

var (
	callHistoryMu sync.RWMutex
	callHistory   []CallRecord
)

// RecordCall appends a call record to the in-memory history.
func RecordCall(rec CallRecord) {
	callHistoryMu.Lock()
	defer callHistoryMu.Unlock()
	callHistory = append(callHistory, rec)

}

// GetCallHistory returns a snapshot copy of the current call history.
// This avoids data races if the caller iterates over the slice.
func GetCallHistory() []CallRecord {
	callHistoryMu.RLock()
	defer callHistoryMu.RUnlock()

	out := make([]CallRecord, len(callHistory))
	copy(out, callHistory)
	return out
}
