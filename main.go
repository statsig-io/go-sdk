package main

import (
	"statsig/internal/evaluation"
	"statsig/pkg/types"
	"time"
)

func main() {
	s := evaluation.New("123")
	time.Sleep(1000 * time.Second)
	s.CheckGate(types.StatsigUser{UserID: "jkw"}, "test_public")
}
