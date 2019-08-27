package main

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/pkg/errors"
	"github.com/yuichiro-h/sfn-status-notifier/config"
	"github.com/yuichiro-h/sfn-status-notifier/log"
	"go.uber.org/zap"
)

type RegistrationExecution struct {
	stopCh     chan bool
	waitStopCh chan bool
}

func NewRegistrationExecution() *RegistrationExecution {
	return &RegistrationExecution{
		stopCh:     make(chan bool),
		waitStopCh: make(chan bool),
	}
}

func (r *RegistrationExecution) Start() {
	log.Get().Info("start registration")

	if err := r.Registration(); err != nil {
		log.Get().Error("failed to registration",
			zap.String("cause", fmt.Sprintf("%+v", err)))
	}

	t := time.NewTicker(time.Second * time.Duration(config.Get().RegistrationInterval))
	for {
		select {
		case <-r.stopCh:
			r.waitStopCh <- true
			return
		case <-t.C:
			if err := r.Registration(); err != nil {
				log.Get().Error("failed to registration",
					zap.String("cause", fmt.Sprintf("%+v", err)))
			}
		}
	}
}

func (r *RegistrationExecution) Registration() error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(config.Get().Region),
	}))

	repo := NewRepository(config.Get().DynamoDBTable, sess)
	lastSearchedAt, err := repo.FindLastSearchedAt()
	if err != nil {
		if err == ErrNotFound {
			now := time.Now()
			lastSearchedAt = &now
		} else {
			return errors.WithStack(err)
		}
	}

	id, err := sts.New(sess).GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return errors.WithStack(err)
	}

	for name := range config.Get().StateMachines {
		in := sfn.ListExecutionsInput{
			StateMachineArn: aws.String(fmt.Sprintf("arn:aws:states:%s:%s:stateMachine:%s",
				config.Get().Region, *id.Account, name)),
		}

		err = sfn.New(sess).ListExecutionsPages(&in, func(o *sfn.ListExecutionsOutput, lastPage bool) bool {
			for _, e := range o.Executions {
				if e.StartDate.Unix() < lastSearchedAt.Unix() {
					return false
				}

				err = repo.CreateExecution(&CreateExecutionInput{
					ExecutionArn: *e.ExecutionArn,
				})
				if err != nil {
					if awsErr, ok := err.(awserr.Error); ok {
						if awsErr.Code() == sfn.ErrCodeStateMachineDoesNotExist {
							log.Get().Warn("not found state machine", zap.Any("name", name))
							continue
						}
					}
					log.Get().Error(err.Error())
					return false
				}
				log.Get().Info("registration execution", zap.Any("execution", e))
			}
			return !lastPage
		})
		if err != nil {
			return errors.WithStack(err)
		}
	}

	if err := repo.UpdateLastSearchedAt(time.Now()); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *RegistrationExecution) Stop() {
	log.Get().Info("stopping registration...")
	r.stopCh <- true
	<-r.waitStopCh
	log.Get().Info("stopped registration")
}
