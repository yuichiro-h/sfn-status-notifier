package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/uuid"
	"github.com/guregu/dynamo"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const shardingNum = 20

var ErrNotFound = errors.New("not found item")

type Repository struct {
	table dynamo.Table
}

func NewRepository(table string, sess *session.Session) *Repository {
	return &Repository{
		table: dynamo.New(sess).Table(table),
	}
}

type LastSearchedAt struct {
	ID             string
	DataType       string
	LastSearchedAt time.Time
}

func (r *Repository) FindLastSearchedAt() (*time.Time, error) {
	var item LastSearchedAt
	err := r.table.
		Get("ID", "sfn-status-notifier").
		Range("DataType", dynamo.Equal, "LastSearchedAt").
		One(&item)

	if err != nil {
		if err == dynamo.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, errors.WithStack(err)
	}

	return &item.LastSearchedAt, nil
}

func (r *Repository) UpdateLastSearchedAt(t time.Time) error {
	err := r.table.Put(&LastSearchedAt{
		ID:             "sfn-status-notifier",
		DataType:       "LastSearchedAt",
		LastSearchedAt: time.Now(),
	}).Run()

	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

type Execution struct {
	ID           string
	DataType     string
	ExecutionArn string
}

type CreateExecutionInput struct {
	ExecutionArn string
}

func (r *Repository) CreateExecution(in *CreateExecutionInput) error {
	rand.Seed(time.Now().UnixNano())
	err := r.table.Put(&Execution{
		ID:           uuid.New().String(),
		DataType:     fmt.Sprintf("Execution-%d", rand.Intn(shardingNum)+1),
		ExecutionArn: in.ExecutionArn,
	}).Run()

	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *Repository) DeleteExecution(e *Execution) error {
	err := r.table.
		Delete("ID", e.ID).
		Range("DataType", e.DataType).
		Run()

	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *Repository) FindAllExecution() ([]Execution, error) {
	var allExecutions []Execution

	eg, ctx := errgroup.WithContext(context.Background())
	m := sync.Mutex{}
	for i := 0; i < shardingNum; i++ {
		i := i
		eg.Go(func() error {
			var executions []Execution
			err := r.table.
				Get("DataType", fmt.Sprintf("Execution-%d", i+1)).
				Index("FindByDataType").
				AllWithContext(ctx, &executions)
			if err != nil {
				return errors.WithStack(err)
			}

			m.Lock()
			allExecutions = append(allExecutions, executions...)
			m.Unlock()

			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, errors.WithStack(err)
	}

	return allExecutions, nil
}
