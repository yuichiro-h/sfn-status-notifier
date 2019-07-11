package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/cenkalti/backoff"
	"github.com/nlopes/slack"
	"github.com/pkg/errors"
	"github.com/yuichiro-h/sfn-status-notifier/config"
	"github.com/yuichiro-h/sfn-status-notifier/log"
	"go.uber.org/zap"
)

type WatchExecution struct {
	stopCh     chan bool
	waitStopCh chan bool
}

func NewWatcherExecution() *WatchExecution {
	return &WatchExecution{
		stopCh:     make(chan bool),
		waitStopCh: make(chan bool),
	}
}

func (r *WatchExecution) Start() {
	log.Get().Info("start watch")

	if err := r.Watch(); err != nil {
		log.Get().Error("failed to watch",
			zap.String("stacktrace", fmt.Sprintf("%+v", err)))
	}

	t := time.NewTicker(time.Second * time.Duration(config.Get().WatchInterval))
	for {
		select {
		case <-r.stopCh:
			r.waitStopCh <- true
			return
		case <-t.C:
			if err := r.Watch(); err != nil {
				log.Get().Error("failed to watch",
					zap.String("stacktrace", fmt.Sprintf("%+v", err)))
			}
		}
	}
}

func (r *WatchExecution) Watch() error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(config.Get().Region),
	}))

	repo := NewRepository(config.Get().DynamoDBTable, sess)
	executions, err := repo.FindAllExecution()
	if err != nil {
		return errors.WithStack(err)
	}
	log.Get().Debug("get watching execution", zap.Int("count", len(executions)))

	sfnCli := sfn.New(sess)
	for _, e := range executions {

		var out *sfn.DescribeExecutionOutput
		err := backoff.Retry(func() error {
			out, err = sfnCli.DescribeExecution(&sfn.DescribeExecutionInput{
				ExecutionArn: aws.String(e.ExecutionArn),
			})
			if err != nil {
				return errors.WithStack(err)
			}
			return nil
		}, backoff.NewExponentialBackOff())

		if err != nil {
			return errors.WithStack(err)
		}

		stateMachineArn := strings.Split(*out.StateMachineArn, ":")
		stateMachineName := stateMachineArn[len(stateMachineArn)-1]

		sm, ok := config.Get().StateMachines[config.StateMachineName(stateMachineName)]
		if !ok {
			continue
		}

		slackConfig := config.Get().Slack
		slackConfig.Merge(sm.Slack)

		link := fmt.Sprintf("https://%s.console.aws.amazon.com/states/home?region=%s#/executions/details/%s",
			config.Get().Region, config.Get().Region, *out.ExecutionArn)

		switch *out.Status {
		case "RUNNING":
			if sm.Deadline == nil {
				continue
			}
			duration := time.Second * time.Duration(*sm.Deadline)
			deadlineTime := out.StartDate.Add(duration)
			if time.Now().Unix() > deadlineTime.Unix() {
				log.Get().Info("execution delayed", zap.String("arn", *out.ExecutionArn))

				_, _, err = slack.New(slackConfig.ApiToken).PostMessage(slackConfig.Channel,
					slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
						Username: slackConfig.Username,
						IconURL:  slackConfig.IconURL,
					}),
					slack.MsgOptionAttachments(slack.Attachment{
						MarkdownIn: []string{"pretext"},
						Pretext:    "Found *DELAYED* execution",
						Color:      slackConfig.AttachmentColor,
						Title:      fmt.Sprintf("%s/%s", stateMachineName, *out.Name),
						TitleLink:  link,
						Fields: []slack.AttachmentField{
							{
								Title: "Start",
								Value: out.StartDate.Format("2006-01-02 15:04"),
								Short: true,
							},
							{
								Title: "Deadline",
								Value: fmt.Sprintf("%s (within %.2f minutes)",
									deadlineTime.Format("2006-01-02 15:04"), duration.Minutes()),
								Short: false,
							},
						},
					}))
				if err != nil {
					return errors.WithStack(err)
				}

				if err := repo.DeleteExecution(&e); err != nil {
					return errors.WithStack(err)
				}
			}
		case "SUCCEEDED", "ABORTED": // ABORTED = Cancel
			log.Get().Info("execution finished",
				zap.String("arn", *out.ExecutionArn),
				zap.String("status", *out.Status))

			if err := repo.DeleteExecution(&e); err != nil {
				return errors.WithStack(err)
			}
		case "FAILED", "TIMED_OUT":
			log.Get().Info("execution failed",
				zap.String("arn", *out.ExecutionArn),
				zap.String("status", *out.Status))

			_, _, err = slack.New(slackConfig.ApiToken).PostMessage(slackConfig.Channel,
				slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
					Username: slackConfig.Username,
					IconURL:  slackConfig.IconURL,
				}),
				slack.MsgOptionAttachments(slack.Attachment{
					MarkdownIn: []string{"pretext"},
					Pretext:    fmt.Sprintf("Found *%s* execution", *out.Status),
					Color:      slackConfig.AttachmentColor,
					Title:      fmt.Sprintf("%s/%s", stateMachineName, *out.Name),
					TitleLink:  link,
					Fields: []slack.AttachmentField{
						{
							Title: "Start",
							Value: out.StartDate.Format("2006-01-02 15:04"),
							Short: true,
						},
						{
							Title: "Stop",
							Value: out.StopDate.Format("2006-01-02 15:04"),
							Short: true,
						},
					},
				}))
			if err != nil {
				return errors.WithStack(err)
			}

			if err := repo.DeleteExecution(&e); err != nil {
				return errors.WithStack(err)
			}
		default:
			return fmt.Errorf("unknown executation status. status=%s", *out.Status)
		}
	}

	return nil
}

func (r *WatchExecution) Stop() {
	log.Get().Info("stopping watch...")
	r.stopCh <- true
	<-r.waitStopCh
	log.Get().Info("stopped watch")
}
