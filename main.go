package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/yuichiro-h/sfn-status-notifier/config"
	"github.com/yuichiro-h/sfn-status-notifier/log"
)

func main() {
	if err := config.Load(os.Getenv("CONFIG_PATH")); err != nil {
		panic(err)
		return
	}
	log.SetConfig(log.Config{
		Debug: config.Get().Debug,
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	r := NewRegistrationExecution()
	w := NewWatcherExecution()
	go r.Start()
	go w.Start()

	<-sigCh
	r.Stop()
	w.Stop()
}
