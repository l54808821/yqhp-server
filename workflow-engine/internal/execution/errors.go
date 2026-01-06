package execution

import "errors"

var (
	// ErrNilConfig 当配置为 nil 时返回。
	ErrNilConfig = errors.New("执行模式配置为 nil")

	// ErrNilIterationFunc 当迭代函数为 nil 时返回。
	ErrNilIterationFunc = errors.New("迭代函数为 nil")

	// ErrNoStages 当递增模式没有定义阶段时返回。
	ErrNoStages = errors.New("递增模式未定义阶段")

	// ErrInvalidRate 当速率无效时返回。
	ErrInvalidRate = errors.New("无效的速率: 必须为正数")

	// ErrInvalidTimeUnit 当时间单位无效时返回。
	ErrInvalidTimeUnit = errors.New("无效的时间单位: 必须为正数")

	// ErrModeNotRunning 当尝试控制未运行的模式时返回。
	ErrModeNotRunning = errors.New("执行模式未运行")

	// ErrModeAlreadyRunning 当尝试启动已运行的模式时返回。
	ErrModeAlreadyRunning = errors.New("执行模式已在运行")

	// ErrScaleNotSupported 当模式不支持扩缩容时返回。
	ErrScaleNotSupported = errors.New("此执行模式不支持扩缩容")

	// ErrPauseNotSupported 当模式不支持暂停时返回。
	ErrPauseNotSupported = errors.New("此执行模式不支持暂停")
)
