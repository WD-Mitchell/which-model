package service

// Event names (D00 CONTRACTS §3, closed enum). No other event name may be
// emitted anywhere in internal/service (B02 SPEC §2.11). TS union mirror:
// packages/core/src/events.ts.
const (
	EventConfigChanged   = "config:changed"
	EventCatalogChanged  = "catalog:changed"
	EventUsageUpdated    = "usage:updated"
	EventSettingsChanged = "settings:changed"
	EventPickRecorded    = "pick:recorded"
)