package elasticsearch

import (
	"fmt"
	"io"
	"net/http"
)

func CalcTotalShardNum(shardNum int, replicaNum int) int {
	return shardNum * (replicaNum + 1)
}

func ExtractError(resp *http.Response) error {
	if resp.StatusCode >= 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("request failed, status: %d", resp.StatusCode)
		}
		return fmt.Errorf("request failed, status: %d, body: %s", resp.StatusCode, body)
	}
	return nil
}
