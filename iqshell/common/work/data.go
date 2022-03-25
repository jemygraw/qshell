package work

type FlowInfo struct {
	WorkerCount       int  // worker 数量
	StopWhenWorkError bool // 当某个 action 遇到执行错误是否结束 batch 任务
	workErrorHappened bool // 执行中是否出现错误
}

func (i *FlowInfo) Check() error {
	if i.WorkerCount <= 0 {
		i.WorkerCount = 1
	}
	return nil
}

type Work interface{}
type Result interface{}