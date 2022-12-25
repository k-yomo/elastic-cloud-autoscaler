package elasticsearch

import "testing"

func TestCalcTotalShardNum(t *testing.T) {
	t.Parallel()

	type args struct {
		shardNum   int
		replicaNum int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "returns total shard count(primary shards + replica shards)",
			args: args{
				shardNum:   4,
				replicaNum: 2,
			},
			want: 12,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalcTotalShardNum(tt.args.shardNum, tt.args.replicaNum); got != tt.want {
				t.Errorf("CalcTotalShardNum() = %v, want %v", got, tt.want)
			}
		})
	}
}
