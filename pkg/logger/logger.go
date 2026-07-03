// Package logger define uma interface mínima de log para que os usecases
// não dependam diretamente do zap (ou qualquer outra lib). Trocar de
// biblioteca de log no futuro não exige alterar a camada de negócio.
package logger

import "go.uber.org/zap"

type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

type zapLogger struct {
	sugar *zap.SugaredLogger
}

func New(env string) (Logger, error) {
	var z *zap.Logger
	var err error
	if env == "production" {
		z, err = zap.NewProduction()
	} else {
		z, err = zap.NewDevelopment()
	}
	if err != nil {
		return nil, err
	}
	return &zapLogger{sugar: z.Sugar()}, nil
}

func (l *zapLogger) Info(msg string, kv ...interface{})  { l.sugar.Infow(msg, kv...) }
func (l *zapLogger) Error(msg string, kv ...interface{}) { l.sugar.Errorw(msg, kv...) }
func (l *zapLogger) Debug(msg string, kv ...interface{}) { l.sugar.Debugw(msg, kv...) }
