package util

import (
	"testing"
)

func TestGetFreePort(t *testing.T) {
	tests := []struct {
		name    string
		want    int
		wantErr bool
	}{
		{name: "getPortCase1", want: 0, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetFreePort()
			t.Logf("got port %d", got)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFreePort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got <= tt.want {
				t.Errorf("GetFreePort() got = %v, want %v", got, tt.want)
			}
		})
	}
}
