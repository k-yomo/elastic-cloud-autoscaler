package timeutil

import (
	"testing"
	"time"
)

func TestMaxDuration(t *testing.T) {
	t.Parallel()

	type args struct {
		a time.Duration
		b time.Duration
	}
	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{
			name: "a is longer than b",
			args: args{a: 2, b: 1},
			want: 2,
		},
		{
			name: "b is longer than a",
			args: args{a: 2, b: 3},
			want: 3,
		},
		{
			name: "equal",
			args: args{a: 2, b: 2},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaxDuration(tt.args.a, tt.args.b); got != tt.want {
				t.Errorf("MaxDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
