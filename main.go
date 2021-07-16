package main

import (
	"statsig/internal/evaluation"
	"statsig/pkg/types"
	"time"
)

func main() {
	s := evaluation.New("secret-9IWfdzNwExEYHEW4YfOQcFZ4xreZyFkbOXHaNbPsMwW")
	s.CheckGate(types.StatsigUser{UserID: "jkw"}, "test_public")
	time.Sleep(2 * time.Second)
	s.CheckGate(types.StatsigUser{UserID: "jkw"}, "test_public")
	time.Sleep(2 * time.Second)
	s.CheckGate(types.StatsigUser{UserID: "jkw"}, "test_public")

	ss := evaluation.New("secret-9IWfdzNwExEYHEW4YfOQcFZ4xreZyFkbOXHaNbPsMwW")

	ss.CheckGate(types.StatsigUser{UserID: "jkw"}, "test_public")
}
