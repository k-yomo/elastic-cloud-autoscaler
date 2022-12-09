package clock

import (
	"testing"
	"time"
)

func TestSetTimeZone(t *testing.T) {
	type args struct {
		timeZone string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "invalid timezone",
			args: args{
				timeZone: "Invalid",
			},
			want:    "Local",
			wantErr: true,
		},
		{
			name: "valid timezone",
			args: args{
				timeZone: "Asia/Tokyo",
			},
			want: "Asia/Tokyo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SetTimeZone(tt.args.timeZone); (err != nil) != tt.wantErr {
				t.Errorf("SetTimeZone() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := time.Local.String(); got != tt.want {
				t.Errorf("SetTimeZone() got = %v, want = %v", got, tt.want)
			}
		})
	}
}
