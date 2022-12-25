package memory

import "testing"

func TestConvertGiBToMiB(t *testing.T) {
	t.Parallel()

	type args struct {
		gb int32
	}
	tests := []struct {
		name string
		args args
		want int32
	}{
		{
			name: "0GiB is 0Mib",
			args: args{
				gb: 0,
			},
			want: 0,
		},
		{
			name: "1GiB is 1024Mib",
			args: args{
				gb: 1,
			},
			want: 1024,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertGiBToMiB(tt.args.gb); got != tt.want {
				t.Errorf("ConvertGiBToMiB() = %v, want %v", got, tt.want)
			}
		})
	}
}
