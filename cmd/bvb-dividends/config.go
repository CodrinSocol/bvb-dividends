package main

import "time"

// Config is the configuration this binary reads that is not already owned by a
// package of its own.
//
// The database, the HTTP server and the logger read their own settings, so what
// is left here is what the composition root itself decides: where BVB is, and
// when to go and ask it.
type Config struct {
	BVB    BVBConfig
	Import ImportConfig
}

// BVBConfig configures the upstream BVB client.
type BVBConfig struct {
	Endpoint string        `env:"BVB_ENDPOINT" envDefault:"https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx" validate:"required,url"`
	Timeout  time.Duration `env:"BVB_TIMEOUT"  envDefault:"30s"                                              validate:"required"`
	Attempts int           `env:"BVB_ATTEMPTS" envDefault:"3"                                                validate:"min=1"`
	Backoff  time.Duration `env:"BVB_BACKOFF"  envDefault:"500ms"                                            validate:"required"`
}

// ImportConfig configures the scheduled import.
type ImportConfig struct {
	// Enabled turns the schedule off, for a deployment that would rather run
	// `bvb-dividends import` from a cron entry or a Kubernetes CronJob.
	Enabled bool `env:"IMPORT_SCHEDULE_ENABLED" envDefault:"true"`

	// Hour is when the import runs, in TimeZone: after the market's morning
	// announcements and well before the close, which is what the Spring service
	// this replaces did.
	Hour int `env:"IMPORT_HOUR" envDefault:"12" validate:"min=0,max=23"`

	// TimeZone is the zone Hour is read in.
	TimeZone string `env:"IMPORT_TIMEZONE" envDefault:"Europe/Bucharest" validate:"required"`

	// RunAtStart imports once on startup rather than waiting for the first
	// scheduled hour. It is off by default, so restarting the service does not
	// hammer BVB.
	RunAtStart bool `env:"IMPORT_RUN_AT_START" envDefault:"false"`

	// Concurrency is how many companies are fetched at once.
	Concurrency int `env:"IMPORT_CONCURRENCY" envDefault:"4" validate:"min=1"`
}
