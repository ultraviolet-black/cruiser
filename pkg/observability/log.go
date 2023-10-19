package observability

import "go.uber.org/zap"

var (
	Log    *zap.SugaredLogger
	RawLog *zap.Logger
)

func InitializeLog() {
	RawLog, _ = zap.NewProduction()

	Log = RawLog.Sugar()
}
