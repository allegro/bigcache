package bigcache

import (
	"testing"
	"time"
)

func TestConfig_valid(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "invalid: shards number",
			cfg:     Config{Shards: 18},
			wantErr: true,
		},
		{
			name:    "invalid: MaxEntriesInWindow",
			cfg:     Config{Shards: 16, MaxEntriesInWindow: -1},
			wantErr: true,
		},
		{
			name:    "invalid: MaxEntrySize",
			cfg:     Config{Shards: 16, MaxEntrySize: -1},
			wantErr: true,
		},
		{
			name:    "invalid: HardMaxCacheSize",
			cfg:     Config{Shards: 16, HardMaxCacheSize: -1},
			wantErr: true,
		},
		{
			name:    "valid",
			cfg:     DefaultConfig(time.Minute),
			wantErr: false,
		},
	} {
		tt := tc
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.valid(); (err != nil) != tt.wantErr {
				t.Errorf("valid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
