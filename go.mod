module github.com/yuichiro-h/sfn-status-notifier

go 1.12

require (
	github.com/aws/aws-sdk-go v1.19.26
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.0 // indirect
	github.com/guregu/dynamo v1.2.1
	github.com/lusis/go-slackbot v0.0.0-20180109053408-401027ccfef5 // indirect
	github.com/lusis/slack-test v0.0.0-20190426140909-c40012f20018 // indirect
	github.com/nlopes/slack v0.5.0
	github.com/pkg/errors v0.8.1
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
	gopkg.in/yaml.v2 v2.2.2
)
