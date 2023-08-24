package interceptor

import "github.com/zeromicro/go-zero/core/logx"

func defaultLogHandler(info *LogInfo) error {
	logx.Infov(*info)
	return nil
}
