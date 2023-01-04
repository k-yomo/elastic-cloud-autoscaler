# elastic-cloud-autoscaler

![License: Apache-2.0](https://img.shields.io/badge/License-MIT-blue.svg)
[![Test](https://github.com/k-yomo/elastic-cloud-autoscaler/actions/workflows/test.yml/badge.svg)](https://github.com/k-yomo/elastic-cloud-autoscaler/actions/workflows/test.yml)
[![Codecov](https://codecov.io/gh/k-yomo/elastic-cloud-autoscaler/branch/main/graph/badge.svg?token=P3pNbMGbeN)](https://codecov.io/gh/k-yomo/elastic-cloud-autoscaler)
[![Go Report Card](https://goreportcard.com/badge/k-yomo/elastic-cloud-autoscaler)](https://goreportcard.com/report/k-yomo/elastic-cloud-autoscaler)

Elastic Cloud Autoscaler based on CPU util or cron schedules inspired by [es-operator](https://github.com/zalando-incubator/es-operator).

**⚠️ This library is still experimental, please use at your own risk if you use.**
I also highly recommend using with `DryRun: true` at first.

## Compatibility
- Elasticsearch >= 8.x

## Features
The autoscaler supports following ways of auto-scaling.
- CPU utilization based auto-scaling.
  - Autoscaler tries to scale-out/scale-in when average CPU util is higher/lower than the desired CPU utilization throughout the threshold duration
- Cron schedule based auto-scaling.
  - You can override min/max node num for configured duration with the cron format schedule.

## Configuration
| Config properties                       | Type            | Required                   | Description                                                                                                                                                             |
|-----------------------------------------|-----------------|----------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| index                                   | string          | true                       | Index to update replicas when scaling out/in                                                                                                                            |
| shardsPerNode                           | int             | true                       | Desired shard count per 1 node. Autoscaler won't scale-in / scale-out to the node count that can't meet this ratio.                                                     |
| defaultMinMemoryGBPerZone               | int             | true                       | Default memory min size per zone.  Available number is only 64,...(64xN node)                                                                                           |
| defaultMaxMemoryGBPerZone               | int             | true                       | Default memory max size per zone.  Available number is only 64,...(64xN node)                                                                                           |
| autoScaling                             | object          |                            |                                                                                                                                                                         |
| autoScaling.desiredCPUUtilPercent       | int             | true (in autoScaling)      | Desired CPU utilization percent. Autoscaler will change nodes to make CPU utilization closer to the desired CPU utilization.                                            |
| autoScaling.scaleOutThresholdDuration   | time.Duration   |                            | Threshold duration for scale-out. When CPU util is higher than desiredCPUUtilPercent throughout the threshold duration scale-out may happen.                            |
| autoScaling.scaleOutCoolDownDuration    | time.Duration   |                            | Cool down period for scale-out after the last scaling operation.                                                                                                        |
| autoScaling.scaleInThresholdDuration    | time.Duration   |                            | Threshold duration for scale-in. When CPU util is lower than desiredCPUUtilPercent throughout the threshold duration scale-in may happen.                               |
| autoScaling.scaleInCoolDownDuration     | time.Duration   |                            | Cool down period for scale-in after the last scaling operation                                                                                                          |
| []scheduledScalings                     | array of object |                            |                                                                                                                                                                         |
| scheduledScalings[i].startCronSchedule  | string          | true (in scheduledScaling) | Cron format schedule to start the specified min/max size. Default timezone is machine local timezone. If you want to specify, set TZ= prefix (e.g. `TZ=UTC 0 0 0 0 0`). |
| scheduledScalings[i].duration           | time.Duration   | true (in scheduledScaling) | Duration to apply above min/max size from startCronSchedule                                                                                                             |
| scheduledScalings[i].minMemoryGBPerZone | int             | true (in scheduledScaling) | Min memory size during the specified period.                                                                                                                            |
| scheduledScalings[i].maxMemoryGBPerZone | int             | true (in scheduledScaling) | Max memory size during the specified period.                                                                                                                            |

### Example YAML Config
```yaml
index: test
shardsPerNode: 1
defaultMinMemoryGBPerZone: 64
defaultMaxMemoryGBPerZone: 256
autoScaling:
  desiredCPUUtilPercent: 50
  scaleOutThresholdDuration: 5m
  scaleInThresholdDuration: 10m
scheduledScalings:
  - startCronSchedule: TZ=UTC 0 0 * * *
    duration: 1h
    minMemoryGBPerZone: 128
    maxMemoryGBPerZone: 256
```

## How it scales
Autoscaler tries to scale-out/scale-in within min/max range keeping configured `shardsPerNode`.

If CPU based auto-scaling is configured, Autoscaler tries to increase/decrease the number of nodes and replicas so that they are closer to the target utilization
when the CPU utilization stays above/below the target CPU utilization for a certain period of time.

If it can't meet the `shardsPerNode`, Autoscaler won't apply scaling operation.

### Example
#### - scale-out when CPU utilization is higher then the target
##### Premise
```yaml
cluster:
  memoryGBPerZone: 384 (64g * 6)
  zoneCount: 2
  averageCPUUtil: 60 (keeping 60 for 5 minutes)
index:
  numberOfShards: 2
  numberOfReplicas: 5
```

##### Config
```yaml
index: test
shardsPerNode: 1
defaultMinMemoryGBPerZone: 284 (64g * 6)
defaultMaxMemoryGBPerZone: 768 (64g * 12)
autoScaling:
  desiredCPUUtilPercent: 45
  scaleOutThresholdDuration: 5m
  scaleInThresholdDuration: 10m
```

##### Result

Scaling-out to 8 nodes per zone to reduce CPU utilization. (`60% * 12 nodes / 16 nodes => 45%`).
```yaml
cluster:
  memoryGBPerZone: 256 (64g * 6) => 512 (64g * 8)
  zoneCount: 2
  averageCPUUtil: 80
index:
  numberOfShards: 2
  numberOfReplicas: 5 => 6
```

#### - not scaling due to `shardsPerNode` violation
##### Premise
```yaml
cluster:
  memoryGBPerZone: 192 (64g * 3)
  zoneCount: 2
index:
  numberOfShards: 3
  numberOfReplicas: 1
```

##### Config
```yaml
index: test
shardsPerNode: 1
defaultMinMemoryGBPerZone: 256 (64g * 4)
defaultMaxMemoryGBPerZone: 256 (64g * 4)
```
In the above case, Autoscaler won't apply scaling-out to 4 nodes x 2 zones even though defaultMinMemoryGBPerZone is 4 nodes.
since either 1 replica (6 shards in total) or 2 replicas (9 shards in total) can't be 8 which is required by `shardsPerNode: 1`.

## Usage
Elastic Cloud Autoscaler can be used as library.
Example is in [./examples/main.go](./examples/main.go).

Also handy docker image is provided. See [kyomo/elastic-cloud-autoscaler](https://hub.docker.com/r/kyomo/elastic-cloud-autoscaler) for more image details.

## Constraints
- Monitoring deployment must be enabled to use this library.
  https://www.elastic.co/guide/en/cloud/current/ec-enable-logging-and-monitoring.html#ec-enable-logging-and-monitoring-steps
- This library only support `hot_content` topology and greater than or equal to `64g` memory size for now.
- Scaling-out from 5 nodes or less to 6 nodes or more is not possible since from 6 data nodes dedicated master nodes are required.
- `auto_expand_replicas` won't be used.  Autoscaler will manually expand / drop replicas before or after node scaling.

## Demo
You can easily test this library with the below repository.
https://github.com/k-yomo/elastic-cloud-autoscaler-demo
